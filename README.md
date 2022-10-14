# DVB-HB Validation and Verification

This repository contains sample code for a DVB-HB server following A179r1 from [DVB-HB](https://dvb.org/?standard=service-discovery-and-delivery-protocols-for-a-dvb-home-broadcast-system)

## Modes
###  static channel map
### dynamic channel map
## Compilation

### Manual compilation

1. select architecture 
    `SET  GOARCH=amd64`
      or 
    `SET  GOARCH=arm64`    
1. select OS
     `SET  GOOS=linux`
     or 
     `SET  GOOS=windows`
1. run build
    `go build`
    
### Continuous Integration
The repository contains automatic build script for [GIT Hub](https://github.com/) in `.github/workflows/build.yaml` . This can be adapted to other build system such as [DroneCI](https://www.drone.io/), [Woodpecker](https://woodpecker-ci.org/) …

## Configuration
Server requires several configuration files to run. 
### democonfig.yaml
This is the main configuration file. The file contains the following fields

####  name (string)
This is the name used by the server to advertise publicly.
#### tunerconfig
Configuration of generic tuner external tool, see [External tool configuration](#exttool) for content detail
#### transcodeconfig
Configuration of generic transcoder external tool, see [External tool configuration](#exttool) for content detail
#### maxtuner (integer)
Maximum number of tuner to use. Setting to 2 will use tuner 0 and 1. Tuner usage can also be specified with tunerlist
#### tunerlist (array of integer)
Gives a list of tuner to use. For instance \[0,2\] will use tuners 0 and 2 (but not 1). 
#### feeds \[string\]string
This is a map used to convert feed name into parameter for tuner. When using external tool the string is passed as in the ${source} parameter in arguments
####  channelmaps
This is list of static channel maps.
## <a name="exttool"></a>External tools
The DVB-HB server can call external tools to perform certains task. The tool is called using the following configuration in YAML
### External tool configuration
#### command (string)
This is the command to run. It can be either just the command name if the tool is in the PATH or a full path.
##### args (string)
Argument passed to the command. Variable in the form of ${name} will be replaced before calling the tool
#### portin (number)
Use a UDP socket to feed data in the tool. The socket port number will be increased with tuner index to avoid port collision.
If no port is specified data are fed to stdin.
#### portout (number)
Use a UDP socket to get data from the tool. The socket port number will be increased with tuner index to avoid port collision.
If no port is specified data grabbed from stdout. In that only stderr output from the tool is dumped to the console.
##### portcommand (number)
Use a UDP socket to send exit command to the tool. The socket port number will be increased with tuner index to avoid port collision.
If no port is specified exit command (if present) is sent to stdin.
#### exitcommand (string)
String to send to the tool to stop it. 
#### mutestdout (boolean)
Prevent stdout from the tool to be sent to the console
#### dummydataonexit (boolean)
Send empty TS packets to tool after sending exit command to force processing of exit command.
## Execution
Just run server from command line

