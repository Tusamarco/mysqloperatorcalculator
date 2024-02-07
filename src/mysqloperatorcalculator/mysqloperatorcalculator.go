package mysqloperatorcalculator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strconv"
)

type MysqlOperatorCalculator struct {
	IncomingRequest ConfigurationRequest
	configurator    Configurator
}

func (moc *MysqlOperatorCalculator) Init(inR ConfigurationRequest) {
	moc.IncomingRequest = inR

}

func (m *MysqlOperatorCalculator) GetSupportedLayouts() Configuration {
	return m.getSupported()
}

// this is the External call to calculate the whole set
func (moc *MysqlOperatorCalculator) GetCalculate() (error, ResponseMessage, map[string]Family) {
	var responseMsg ResponseMessage
	var families map[string]Family
	var ConfRequest ConfigurationRequest

	ConfRequest = moc.IncomingRequest

	// Before going to the configurator we check the incoming request and IF is not ok we return an error message
	if ConfRequest.Dimension.Id == 0 || ConfRequest.LoadType.Id == 0 {
		err := fmt.Errorf("Possible Malformed request, Dimension ID: %d; LoadType ID: %d", ConfRequest.Dimension.Id, ConfRequest.LoadType.Id)
		if err != nil {
			return err, responseMsg, families
		}
		return nil, responseMsg, families
	} else if ConfRequest.Dimension.Id == DimensionOpen && (ConfRequest.Dimension.Cpu == 0 || ConfRequest.Dimension.MemoryBytes == 0) {
		err := fmt.Errorf("Open dimension request missing CPU OR Memory value CPU: %g, Memory %g", ConfRequest.Dimension.Cpu, ConfRequest.Dimension.Memory)
		if err != nil {
			return err, responseMsg, families
		}
		return nil, responseMsg, families

	} else if ConfRequest.DBType != DbTypePXC && ConfRequest.DBType != DbTypeGroupReplication {
		err := fmt.Errorf("DB Type is not correct Supported Types are : %s, %s ", DbTypePXC, DbTypeGroupReplication)
		if err != nil {
			return err, responseMsg, families
		}
		return nil, responseMsg, families

	}

	// Internally if Connection dimension is NOT passed we will loop in a very rude way to calculate the maximum
	// number of supported calculation

	error, message, Families := moc.getCalculateInt()

	if moc.IncomingRequest.Connections == 0 {
		moc.IncomingRequest.Connections = 50
		for message.MType != OverutilizingI {
			moc.IncomingRequest.Connections = moc.IncomingRequest.Connections + 10
			error, message, Families = moc.getCalculateInt()
		}

		moc.IncomingRequest.Connections = moc.IncomingRequest.Connections - 10
		error, message, Families = moc.getCalculateInt()
	}

	if message.MType == OverutilizingI {
		originalConnections := moc.IncomingRequest.Connections
		for message.MType == OverutilizingI {
			moc.IncomingRequest.Connections = moc.IncomingRequest.Connections - 10
			//log.Debug("Recalculating connections " + strconv.Itoa(moc.IncomingRequest.Connections))
			error, message, Families = moc.getCalculateInt()
		}
		message.MText = message.MText + "\n!!!! Connections recalculated Original: " + strconv.Itoa(originalConnections) + " New Value " + strconv.Itoa(moc.IncomingRequest.Connections) + " plus additional 2 for administrative use !!!\n\n"
		message.MName = message.GetMessageText(ConnectionRecalculated)
		message.MType = ConnectionRecalculated
	}

	return error, message, Families
}

func (moc *MysqlOperatorCalculator) getCalculateInt() (error, ResponseMessage, map[string]Family) {
	// message object to pass back
	var responseMsg ResponseMessage
	var family Family
	var conf Configuration
	var families map[string]Family
	var ConfRequest ConfigurationRequest

	ConfRequest = moc.IncomingRequest

	conf.Init()
	ConfRequest = getConfForConfRequest(ConfRequest, conf)
	families = family.Init(ConfRequest.DBType)

	responseMsg, connectionsOverload := moc.configurator.Init(ConfRequest, families, conf, responseMsg)

	if connectionsOverload {
		responseMsg.MName = "Resources Overload"
		responseMsg.MText = "Too many connections for the chose dimension. Resource Overload, decrease number of connections OR choose higher CPUs value"
		families = make(map[string]Family)
	} else {
		// here is the calculation step
		overUtilizing := false
		moc.configurator.ProcessRequest()
		responseMsg, overUtilizing = moc.configurator.EvaluateResources(responseMsg)

		//if request overutilize resources WE DO NOT pass params but message
		if overUtilizing {
			families = make(map[string]Family)
			err := fmt.Errorf(strconv.Itoa(responseMsg.MType) + ": " + responseMsg.MName + " " + responseMsg.MText)
			if err != nil {
				return err, responseMsg, families
			}
		}
	}

	//fmt.Fprintf(os.Stdout, "\n%s\n", body)

	return nil, responseMsg, families
}

// we loop the arrays to get all the info we may need for the operation using the ID as reference
func getConfForConfRequest(request ConfigurationRequest, conf Configuration) ConfigurationRequest {

	if request.Dimension.Id != DimensionOpen {
		for i := 0; i < len(conf.Dimension); i++ {

			if request.Dimension.Id == conf.Dimension[i].Id {
				request.Dimension = conf.Dimension[i]
				break
			}
		}
	} else {
		//We need to calibrate the dimension on the base of an open request
		request.Dimension = conf.CalculateOpenDimension(request.Dimension)
	}

	for i := 0; i < len(conf.LoadType); i++ {

		if request.Dimension.Id == conf.LoadType[i].Id {
			request.LoadType = conf.LoadType[i]
			break
		}
	}

	return request
}

func ReturnResponse(writer http.ResponseWriter, request *http.Request, ConfRequest *ConfigurationRequest, message ResponseMessage, families map[string]Family) error {
	var b bytes.Buffer
	//var err error

	//if ConfRequest.Output == "json" {
	//	b, err = GetJSONOutput(message, ConfRequest, families)
	//} else {
	//	b, err = GetHumanOutput(message, ConfRequest, families)
	//}
	//if err != nil {
	//	return err
	//}

	// Return the information
	writer.Header().Set("Content/Type", "application/json")
	writer.Write(b.Bytes())
	return nil
}

func (moc *MysqlOperatorCalculator) GetHumanOutput(message ResponseMessage, request ConfigurationRequest, families map[string]Family) (bytes.Buffer, error) {
	var b bytes.Buffer
	var err error
	// process one section a time
	b.WriteString("[message]\n")
	b.WriteString("name = " + message.MName + "\n")
	b.WriteString("type = " + strconv.Itoa(message.MType) + "\n")
	b.WriteString("text = " + message.MText + "\n")

	family, _ := families[FamilyTypeMysql]
	fb := family.ParseGroupsHuman()
	b.Write(fb.Bytes())

	family, _ = families[FamilyTypeProxy]
	fb = family.ParseGroupsHuman()
	b.Write(fb.Bytes())

	family, _ = families[FamilyTypeMonitor]
	fb = family.ParseGroupsHuman()
	b.Write(fb.Bytes())

	return b, err
}

func (moc *MysqlOperatorCalculator) GetJSONOutput(message ResponseMessage, ConfRequest ConfigurationRequest, families map[string]Family) (bytes.Buffer, error) {
	//output, err := json.Marshal(&conf)
	messageStream, err := json.MarshalIndent(message, "", "  ")
	// we store incoming request for reference when passing back the configuration
	processedRequest, err := json.MarshalIndent(&ConfRequest, "", "  ")

	// We transform to Json all the calculated params
	output, err := json.MarshalIndent(&families, "", "  ")

	// Concatenate all into a single output
	var b bytes.Buffer
	b.WriteString(`{"request": {`)
	b.WriteString(`,"message":`)
	b.Write(messageStream)
	b.WriteString(`,"incoming":`)
	b.Write(processedRequest)
	b.WriteString(`,"answer":`)
	b.Write(output)
	b.WriteString("}}")
	if err != nil {
		return b, err
	}
	return b, err
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
	err := ReturnResponse(writer, request, ConfRequest, message, families)
	if err != nil {
		return err
	}

	return nil
}

//=====================================================
//Families section
//=====================================================

func (moc *MysqlOperatorCalculator) GetFamily(familyname string) (Family, error) {
	var family Family
	var err1 error

	if familyname == FamilyTypeMysql ||
		familyname == FamilyTypeProxy ||
		familyname == FamilyTypeMonitor {
		return moc.configurator.families[familyname], err1
	}
	err1 = errors.New("ERROR: Invalid Family name")
	return family, err1
}
