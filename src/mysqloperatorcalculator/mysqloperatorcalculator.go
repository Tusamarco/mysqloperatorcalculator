package mysqloperatorcalculator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"code.cloudfoundry.org/bytefmt"
	log "github.com/sirupsen/logrus"
)

type MysqlOperatorCalculator struct {
	IncomingRequest ConfigurationRequest
	configurator    Configurator
	Conf            Configuration
}

func (moc *MysqlOperatorCalculator) Init(inR ConfigurationRequest, conf Configuration) ConfigurationRequest {
	moc.IncomingRequest = inR
	moc.Conf = conf
	if moc.IncomingRequest.ProviderCostPct > 0 {
		moc.adjustResourcesByProvider()
	}
	moc.GetConfForConfRequest()
	return moc.IncomingRequest
}

func (m *MysqlOperatorCalculator) GetSupportedLayouts() Configuration {
	return m.getSupported()
}

// GetCalculate is the external call to calculate the whole set
func (moc *MysqlOperatorCalculator) GetCalculate() (error, ResponseMessage, map[string]Family) {
	var responseMsg ResponseMessage
	var families map[string]Family
	var ConfRequest = moc.IncomingRequest
	calculateByConnection := false

	// Check incoming request; return an error immediately if malformed
	if ConfRequest.Dimension.Id == 0 || ConfRequest.LoadType.Id == 0 {
		return fmt.Errorf("Possible Malformed request, Dimension ID: %d; LoadType ID: %d", ConfRequest.Dimension.Id, ConfRequest.LoadType.Id), responseMsg, families
	} else if ConfRequest.Dimension.Id == DimensionOpen && (ConfRequest.Dimension.Cpu == 0 || ConfRequest.Dimension.MemoryBytes == 0) {
		return fmt.Errorf("Open dimension request missing CPU OR Memory value CPU: %g, Memory %g", ConfRequest.Dimension.Cpu, ConfRequest.Dimension.Memory), responseMsg, families
	}

	if ConfRequest.DBType != DbTypePXC && ConfRequest.DBType != DbTypeGroupReplication {
		return fmt.Errorf("DB Type is not correct. Supported Types are: %s, %s", DbTypePXC, DbTypeGroupReplication), responseMsg, families
	}

	// If calculating by connection (id = 998) and valid number for connection
	if ConfRequest.Dimension.Id == 998 {
		if moc.IncomingRequest.Connections < MinConnectionNumber {
			moc.IncomingRequest.Connections = MinConnectionNumber
		}
		log.Info("Calculating by number of connections")
		moc.IncomingRequest.Dimension = moc.Conf.Dimension[0]
		calculateByConnection = true
	}

	calcErr, message, Families := moc.getCalculateInt()

	// Calculate the resources by the number of given connections
	if calculateByConnection {
		calcErr, message, Families = moc.getCalculateInt()
		for message.MType == OverutilizingI {
			dimension, _ := moc.Conf.ScaleDimension(moc.IncomingRequest.Dimension)
			moc.IncomingRequest.Dimension = dimension
			calcErr, message, Families = moc.getCalculateInt()
		}
		message.MText += "\n!!!! Resources calculated to match connections request\n\n"
		message.MName = message.GetMessageText(ResourcesRecalculated)
		message.MType = ResourcesRecalculated
	}

	// Auto calculation of the connections
	if moc.IncomingRequest.Connections == 0 {
		moc.IncomingRequest.Connections = MinConnectionNumber
		for message.MType != OverutilizingI {
			moc.IncomingRequest.Connections += 10
			calcErr, message, Families = moc.getCalculateInt()
		}

		moc.IncomingRequest.Connections -= 10
		calcErr, message, Families = moc.getCalculateInt()
	}

	if message.MType == OverutilizingI {
		originalConnections := moc.IncomingRequest.Connections
		for message.MType == OverutilizingI && moc.IncomingRequest.Connections > MinConnectionNumber {
			moc.IncomingRequest.Connections -= 10
			calcErr, message, Families = moc.getCalculateInt()
		}
		message.MText += fmt.Sprintf("\n!!!! Connections recalculated Original: %d New Value %d plus additional 2 for administrative use !!!\n\n", originalConnections, moc.IncomingRequest.Connections)
		message.MName = message.GetMessageText(ConnectionRecalculated)
		message.MType = ConnectionRecalculated
	}

	return calcErr, message, Families
}

func (moc *MysqlOperatorCalculator) getCalculateInt() (error, ResponseMessage, map[string]Family) {
	var responseMsg ResponseMessage
	var family Family
	var conf Configuration

	ConfRequest := moc.IncomingRequest
	conf.Init()

	// Handle pre-defined configs
	if ConfRequest.Dimension.Id != 999 && ConfRequest.Dimension.Id != 998 && ConfRequest.Dimension.Name != "scaled" {
		ConfRequest = getConfForConfRequest(ConfRequest, conf)
	}

	families := family.Init(ConfRequest.DBType)
	responseMsg, connectionsOverload := moc.configurator.Init(ConfRequest, families, conf, responseMsg)

	if connectionsOverload {
		responseMsg.MName = "Resources Overload"
		responseMsg.MText = "Too many connections for the chosen dimension. Resource Overload, decrease number of connections OR choose higher CPUs value"
		families = make(map[string]Family)
	} else {
		overUtilizing := false
		moc.configurator.ProcessRequest()
		responseMsg, overUtilizing = moc.configurator.EvaluateResources(responseMsg)

		if overUtilizing {
			families = make(map[string]Family)
			return fmt.Errorf("%d: %s %s", responseMsg.MType, responseMsg.MName, responseMsg.MText), responseMsg, families
		}
	}

	return nil, responseMsg, families
}

func getConfForConfRequest(request ConfigurationRequest, conf Configuration) ConfigurationRequest {
	if request.Dimension.Id != DimensionOpen {
		for i := range conf.Dimension {
			if request.Dimension.Id == conf.Dimension[i].Id {
				request.Dimension = conf.Dimension[i]
				break
			}
		}
	} else {
		request.Dimension = conf.CalculateOpenDimension(request.Dimension)
	}

	for i := range conf.LoadType {
		if request.LoadType.Id == conf.LoadType[i].Id {
			request.LoadType = conf.LoadType[i]
			break
		}
	}

	return request
}

func ReturnResponse(writer http.ResponseWriter, request *http.Request, ConfRequest *ConfigurationRequest, message ResponseMessage, families map[string]Family) error {
	var b bytes.Buffer

	// NOTE: You had formatting logic commented out here.
	// If `b` remains empty, writer.Write(b.Bytes()) will write nothing.

	writer.Header().Set("Content-Type", "application/json")
	writer.Write(b.Bytes())
	return nil
}

func (moc *MysqlOperatorCalculator) GetHumanOutput(message ResponseMessage, request ConfigurationRequest, families map[string]Family) (bytes.Buffer, error) {
	var b bytes.Buffer

	b.WriteString("[message]\n")
	b.WriteString("name = " + message.MName + "\n")
	b.WriteString("type = " + strconv.Itoa(message.MType) + "\n")
	b.WriteString("text = " + message.MText + "\n")

	if family, ok := families[FamilyTypeMysql]; ok {
		my := family.ParseGroupsHuman()
		b.Write(my.Bytes())
	}
	if family, ok := families[FamilyTypeProxy]; ok {
		proxy := family.ParseGroupsHuman()
		b.Write(proxy.Bytes())
	}
	if family, ok := families[FamilyTypeMonitor]; ok {
		mon := family.ParseGroupsHuman()
		b.Write(mon.Bytes())
	}

	return b, nil
}

func (moc *MysqlOperatorCalculator) GetJSONOutput(message ResponseMessage, ConfRequest ConfigurationRequest, families map[string]Family) (bytes.Buffer, error) {
	var b bytes.Buffer

	// Combine data into a single structured map/struct to marshal cleanly
	responsePayload := struct {
		Request struct {
			Message  ResponseMessage      `json:"message"`
			Incoming ConfigurationRequest `json:"incoming"`
			Answer   map[string]Family    `json:"answer"`
		} `json:"request"`
	}{}

	responsePayload.Request.Message = message
	responsePayload.Request.Incoming = ConfRequest
	responsePayload.Request.Answer = families

	output, err := json.MarshalIndent(responsePayload, "", "  ")
	if err != nil {
		return b, err
	}

	b.Write(output)
	return b, nil
}

func (moc *MysqlOperatorCalculator) getSupported() Configuration {
	var conf Configuration
	conf.Init()
	return conf
}

func exitWithCode(errorCode int) {
	log.Debug("Exiting execution with code ", errorCode)
	os.Exit(errorCode)
}

func returnErrorMessage(writer http.ResponseWriter, request *http.Request, ConfRequest *ConfigurationRequest, message ResponseMessage, families map[string]Family, errorMessage string) error {
	message.MType = ErrorexecI
	message.MName = "Invalid incoming request"
	message.MText = fmt.Sprintf(message.GetMessageText(message.MType), errorMessage)
	return ReturnResponse(writer, request, ConfRequest, message, families)
}

// GetFamily retrieves the Family object corresponding to the given family name or returns an error if the name is invalid.
func (moc *MysqlOperatorCalculator) GetFamily(familyname string) (Family, error) {
	if familyname == FamilyTypeMysql || familyname == FamilyTypeProxy || familyname == FamilyTypeMonitor {
		return moc.configurator.families[familyname], nil
	}
	return Family{}, errors.New("ERROR: Invalid Family name")
}

// GetConfForConfRequest updates the incoming request configuration by applying matching dimensions and load types.
func (moc *MysqlOperatorCalculator) GetConfForConfRequest() {
	if moc.IncomingRequest.Dimension.Id != DimensionOpen {
		for i := range moc.Conf.Dimension {
			if moc.IncomingRequest.Dimension.Id == moc.Conf.Dimension[i].Id {
				moc.IncomingRequest.Dimension = moc.Conf.Dimension[i]
				break
			}
		}
	} else {
		moc.IncomingRequest.Dimension = moc.Conf.CalculateOpenDimension(moc.IncomingRequest.Dimension)
	}

	for i := range moc.Conf.LoadType {
		// BUG FIX: Originally compared `Dimension.Id` against `LoadType[i].Id`.
		if moc.IncomingRequest.LoadType.Id == moc.Conf.LoadType[i].Id {
			moc.IncomingRequest.LoadType = moc.Conf.LoadType[i]
			break
		}
	}
}

func (moc *MysqlOperatorCalculator) adjustResourcesByProvider() {
	moc.IncomingRequest.Dimension.Cpu = int(float64(moc.IncomingRequest.Dimension.Cpu) * (1.0 - moc.IncomingRequest.ProviderCostPct))
	moc.IncomingRequest.Dimension.MemoryBytes = float64(moc.IncomingRequest.Dimension.MemoryBytes) * (1.0 - moc.IncomingRequest.ProviderCostPct)
	moc.IncomingRequest.Dimension.Memory = bytefmt.ByteSize(uint64(moc.IncomingRequest.Dimension.MemoryBytes))
}
