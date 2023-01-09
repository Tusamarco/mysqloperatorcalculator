# MySQL Calculator for Operator
## Why 
With the advent of Kubernets (k8), had become incresingly common to deploy RBDMS on K8 supported platforms. 
However the way MySQL and also the other components should be set and tune is very different from what is the "standard" way. 
To facilitate the setup and configuration of MYSQL and related, I have wrote this small tool that works as a simple service and that can be query directly 
from your application.

## How 
The tools is a simple service that will listen wherever you run it. 
The calculation is done considering many different parameters combinations. 
The Parameters are:
- Dimensions (CPU/Memory)
- Kind of load (simple reads with very minimal writes say less than 5%; still reads but higher writes less 20%; kind of 50/50% load in reads and writes).
- Number of connections

While the fisrst two are fix and passed by the tool, the number of connection is an open variable, and you can set it to any number considering the minum as _50 connections_. 
It doesn't make too much sense to have a RDBMS with less than that, don't you think? 


### What I should do 
Ok, so what should I do to run it?
After compilation run it as `
The first action is to discover what is currently 
