package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"runtime"
	"time"

	"flexbot/pkg/config"
	"flexbot/pkg/ipam"
	"flexbot/pkg/ontap"
	"flexbot/pkg/ucsm"
	"flexbot/pkg/util/crypt"
	"github.com/denisbrodbeck/machineid"
	"gopkg.in/yaml.v3"
)

const (
	version = "1.6.2"
)

type OperationResult interface {
	DumpResult(r interface{}, resultDest string, resultFormat string, resultErr error)
}

type BaseResult struct {
	Status       string `yaml:"status" json:"status"`
	ErrorMessage string `yaml:"errorMessage,omitempty" json:"errorMessage,omitempty"`
}

type NodeResult struct {
	BaseResult              `yaml:",inline" json:",inline"`
	Node *config.NodeConfig `yaml:"server,omitempty" json:"server,omitempty"`
}

type ImageResult struct {
	BaseResult      `yaml:",inline" json:",inline"`
	Images []string `yaml:"images,omitempty" json:"images,omitempty"`
}

type TemplateResult struct {
	BaseResult         `yaml:",inline" json:",inline"`
	Templates []string `yaml:"templates,omitempty" json:"templates,omitempty"`
}

type SnapshotResult struct {
	BaseResult         `yaml:",inline" json:",inline"`
	Snapshots []string `yaml:"snapshots,omitempty" json:"snapshots,omitempty"`
}

func usage() {
	goOS := runtime.GOOS
	goARCH := runtime.GOARCH
	fmt.Printf("flexbot version %s %s/%s\n\n", version, goOS, goARCH)
	flag.Usage()
	fmt.Println("")
	fmt.Printf("flexbot --config=<config file path> --op=provisionServer --host=<host name> --image=<image name> --templatePath=<cloud-init template path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=stopServer --host=<host name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=startServer --host=<host name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deprovisionServer --host=<host name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=uploadImage --image=<image name> --imagePath=<image path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deleteImage --image=<image name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=listImages\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=uploadTemplate --template=<template name> --templatePath=<template path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=downloadTemplate --template=<template name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deleteTemplate --template=<template name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=listTemplates\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=createSnapshot --host=<host name> --snapshot=<snapshot name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deleteSnapshot --host=<host name> --snapshot=<snapshot name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=restoreSnapshot --host=<host name> --snapshot=<snapshot name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=listSnapshots --host=<host name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=decryptConfig [--passphrase=<password phrase>]\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=encryptConfig [--passphrase=<password phrase>]\n\n")
	fmt.Printf("flexbot --op=encryptString --sourceString <string to encrypt> [--passphrase=<password phrase>]\n\n")
	fmt.Printf("flexbot --version\n\n")
}

func printProgess(done <-chan bool) {
	for {
		select {
		case res, valid := <-done:
			if res && valid {
				fmt.Println("success")
				return
			} else {
				fmt.Println("failure")
				return
			}
		default:
			time.Sleep(1 * time.Second)
			fmt.Print(".")
		}
	}
}

func (result *BaseResult) DumpResult(r interface{}, resultDest string, resultFormat string, resultErr error) {
	var b []byte
	var err error
	if resultErr == nil {
		result.Status = "success"
	} else {
		result.Status = "failure"
		result.ErrorMessage = resultErr.Error()
	}
	if resultFormat == "yaml" {
		b, err = yaml.Marshal(r)
	} else {
		b, err = json.Marshal(r)
	}
	if err != nil {
		panic("Failure to decode operation result: " + err.Error())
	} else {
		if resultDest == "STDOUT" {
			fmt.Print(string(b))
		} else {
			if err = ioutil.WriteFile(resultDest, b, 0644); err != nil {
				panic("Failure to write image result: " + err.Error())
			}
		}
	}
        return
}

func (result *NodeResult) DumpResult(r interface{}, resultDest string, resultFormat string, resultErr error) {
	result.Node.Ipam.IbCredentials = config.InfobloxCredentials{}
	result.Node.Storage.CdotCredentials = config.CdotCredentials{}
	result.Node.Compute.UcsmCredentials = config.Credentials{}
	result.Node.CloudArgs = map[string]string{}
	result.BaseResult.DumpResult(r, resultDest, resultFormat, resultErr)
	return
}

func provisionServer(nodeConfig *config.NodeConfig) (err error) {
	var ipamProvider ipam.IpamProvider
	if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
		return
	}
	if err = ipamProvider.Allocate(nodeConfig); err != nil {
		return
	}
	if err = ontap.CreateBootStorage(nodeConfig); err != nil {
		return
	}
	if _, err = ucsm.CreateServer(nodeConfig); err != nil {
		return
	}
	if err = ontap.CreateSeedStorage(nodeConfig); err != nil {
		return
	}
	if err = ucsm.StartServer(nodeConfig); err != nil {
		return
	}
	return
}

func discoverServer(nodeConfig *config.NodeConfig) (serverExists bool, err error) {
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		return
	}
	if serverExists {
		var ipamProvider ipam.IpamProvider
		if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err == nil {
			if err = ipamProvider.Discover(nodeConfig); err != nil {
				return
			}
		}
	}
	return
}

func provisionServerPreflight(nodeConfig *config.NodeConfig) (err error) {
	var stepErr error
	var ipamProvider ipam.IpamProvider
	if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
		return
	}
	if stepErr = ipamProvider.AllocatePreflight(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	if stepErr = ontap.CreateBootStoragePreflight(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	if stepErr = ucsm.CreateServerPreflight(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	if stepErr = ontap.CreateSeedStoragePreflight(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	return
}

func deprovisionServer(nodeConfig *config.NodeConfig) (err error) {
	var stepErr error
	var powerState string
	var ipamProvider ipam.IpamProvider
	if ipamProvider, err = ipam.NewProvider(&nodeConfig.Ipam); err != nil {
		return
	}
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		return
	} else {
		if powerState == "up" {
			err = fmt.Errorf("DeprovisionServer: server \"%s\" has power state \"%s\"", nodeConfig.Compute.HostName, powerState)
			return
		}
	}
	if stepErr = ucsm.DeleteServer(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	if stepErr = ontap.DeleteBootStorage(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	if stepErr = ipamProvider.Release(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	return
}

func restoreSnapshot(nodeConfig *config.NodeConfig, snapshotName string) (err error) {
	var powerState string
	if powerState, err = ucsm.GetServerPowerState(nodeConfig); err != nil {
		return
	} else {
		if powerState == "up" {
			err = fmt.Errorf("RestoreSnapshot: cannot restore LUNs from snapshot, server \"%s\" has power state \"%s\"", nodeConfig.Compute.HostName, powerState)
			return
		}
	}
	err = ontap.RestoreSnapshot(nodeConfig, snapshotName)
	return
}

func uploadImage(nodeConfig *config.NodeConfig, imageName string, imagePath string) (err error) {
	outcome := make(chan bool)
	defer close(outcome)
	fmt.Printf("Uploading image..")
	go printProgess(outcome)
	if err = ontap.CreateRepoImage(nodeConfig, imageName, imagePath); err != nil {
		outcome <- false
	} else {
		outcome <- true
	}
	time.Sleep(1 * time.Second)
	return
}

func uploadTemplate(nodeConfig *config.NodeConfig, templateName string, templatePath string) (err error) {
	outcome := make(chan bool)
	defer close(outcome)
	fmt.Printf("Uploading template..")
	go printProgess(outcome)
	if err = ontap.CreateRepoTemplate(nodeConfig, templateName, templatePath); err != nil {
		outcome <- false
	} else {
		outcome <- true
	}
	time.Sleep(1 * time.Second)
	return
}

func dumpNodeConfig(configDest string, nodeConfig *config.NodeConfig, format string) {
	var b []byte
	var err error
	if format == "yaml" {
		b, err = yaml.Marshal(nodeConfig)
	} else {
		b, err = json.Marshal(nodeConfig)
	}
	if err != nil {
		panic("Failure to marshal node config: " + err.Error())
	} else {
		if configDest == "STDOUT" {
			fmt.Print(string(b))
		} else {
			if err = ioutil.WriteFile(configDest, b, 0644); err != nil {
				panic("Failure to write node config: " + err.Error())
			}
		}
	}
}

func encryptString(srcString string, passPhrase string) (encrypted string, err error) {
	var b []byte
	if b, err = crypt.Encrypt([]byte(srcString), passPhrase); err != nil {
		err = fmt.Errorf("EncryptString: Encrypt() failure: %s", err)
	} else {
		encrypted = "base64:" + base64.StdEncoding.EncodeToString(b)
	}
	return
}

func main() {
	var nodeConfig config.NodeConfig
	var err error
	var passPhrase string
	optHostName := flag.String("host", "", "compute node name")
	optImageName := flag.String("image", "", "boot image name")
	optImagePath := flag.String("imagePath", "", "a path to boot image (prefix can be either file:// or http(s)://)")
	optTemplateName := flag.String("template", "", "cloud-init template name or path (prefix can be either file:// or http(s)://)")
	optTemplatePath := flag.String("templatePath", "", "cloud-init template path (prefix can be either file:// or http(s)://)")
	optSnapshotName := flag.String("snapshot", "", "volume snapshot name")
	optPassPhrase := flag.String("passphrase", "", "passphrase to encrypt/decrypt passwords in configuration (default is machineid)")
	optSourceString := flag.String("sourceString", "", "source string to encrypt")
	optNodeConfig := flag.String("config", "STDIN", "a path to configuration file, STDIN, or argument value in JSON")
	optOp := flag.String("op", "", "operation: \n\tprovisionServer\n\tdeprovisionServer\n\tstopServer\n\tstartServer\n\tuploadImage\n\tdeleteImage\n\tlistImages\n\tuploadTemplate\n\tdownloadTemplate\n\tdeleteTemplate\n\tlistTemplates\n\tcreateSnapshot\n\tdeleteSnapshot\n\trestoreSnapshot\n\tlistSnapshots\n\tencryptConfig\n\tdecryptConfig\n\tencryptString")
	optDumpResult := flag.String("dumpResult", "STDOUT", "dump result: file path or STDOUT")
	optEncodingFormat := flag.String("encodingFormat", "yaml", "supported encoding formats: json, yaml")
	optVersion := flag.Bool("version", false, "flexbot version")
	flag.Parse()
	if *optVersion {
		goOS := runtime.GOOS
		goARCH := runtime.GOARCH
		fmt.Printf("flexbot version %s %s/%s\n", version, goOS, goARCH)
		return
	}
	if *optPassPhrase == "" {
		if passPhrase, err = machineid.ID(); err != nil {
			err = fmt.Errorf("main() failure to get machine ID: %s", err)
			panic(err.Error())
		}
	} else {
		passPhrase = *optPassPhrase
	}
	if !(*optOp == "encryptString" || *optOp == "") {
		if err = config.ParseNodeConfig(*optNodeConfig, &nodeConfig); err != nil {
			err = fmt.Errorf("ParseNodeConfig() failure: %s", err)
			panic(err.Error())
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, *optImageName, *optTemplateName, passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			panic(err.Error())
		}
	}
	switch *optOp {
	case "provisionServer":
		var nodeResult OperationResult = &NodeResult{Node: &nodeConfig}
		if nodeConfig.Compute.HostName == "" || nodeConfig.Storage.BootLun.OsImage.Name == "" || nodeConfig.Storage.SeedLun.SeedTemplate.Location == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName, storage.bootLun.osImage.name, and storage.seedLun.seedTemplate.location")
		} else {
			var serverExists bool
			if serverExists, err = discoverServer(&nodeConfig); err == nil {
				if serverExists == false {
					if err = provisionServerPreflight(&nodeConfig); err == nil {
						if err = provisionServer(&nodeConfig); err != nil {
							deprovisionServer(&nodeConfig)
						}
					}
				}
			}
		}
		nodeResult.DumpResult(nodeResult, *optDumpResult, *optEncodingFormat, err)
	case "deprovisionServer":
		var nodeResult OperationResult = &NodeResult{Node: &nodeConfig}
		if nodeConfig.Compute.HostName == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName")
		} else {
			err = deprovisionServer(&nodeConfig)
		}
		nodeResult.DumpResult(nodeResult, *optDumpResult, *optEncodingFormat, err)
	case "stopServer":
		var nodeResult OperationResult = &NodeResult{Node: &nodeConfig}
		if nodeConfig.Compute.HostName == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName")
		} else {
			err = ucsm.StopServer(&nodeConfig)
		}
		nodeResult.DumpResult(nodeResult, *optDumpResult, *optEncodingFormat, err)
	case "startServer":
		var nodeResult OperationResult = &NodeResult{Node: &nodeConfig}
		if nodeConfig.Compute.HostName == "" {
			err = fmt.Errorf("main() failure: expected image name and image path")
		} else {
			err = ucsm.StartServer(&nodeConfig)
		}
		nodeResult.DumpResult(nodeResult, *optDumpResult, *optEncodingFormat, err)
	case "uploadImage":
		var baseResult OperationResult = &BaseResult{}
		if *optImageName == "" || *optImagePath == "" {
			err = fmt.Errorf("main() failure: expected image name and image path")
			baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
		} else {
			if err = uploadImage(&nodeConfig, *optImageName, *optImagePath); err != nil {
				baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
			}
		}
	case "uploadTemplate":
		var baseResult OperationResult = &BaseResult{}
		if *optTemplateName == "" || *optTemplatePath == "" {
			err = fmt.Errorf("main() failure: expected template name and template path")
			baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
		} else {
			if err = uploadTemplate(&nodeConfig, *optTemplateName, *optTemplatePath); err != nil {
				baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
			}
		}
	case "downloadTemplate":
		var baseResult OperationResult = &BaseResult{}
		var templateContent []byte
		if *optTemplateName == "" {
			err = fmt.Errorf("main() failure: expected template name")
			baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
		} else {
			if templateContent, err = ontap.DownloadRepoTemplate(&nodeConfig, *optTemplateName); err == nil {
				fmt.Print(string(templateContent))
			} else {
				baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
			}
		}
	case "deleteImage":
		var baseResult OperationResult = &BaseResult{}
		if *optImageName == "" {
			err = fmt.Errorf("main() failure: expected image name")
		} else {
			err = ontap.DeleteRepoImage(&nodeConfig, *optImageName)
		}
		baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
	case "deleteTemplate":
		var baseResult OperationResult = &BaseResult{}
		if *optTemplateName == "" {
			err = fmt.Errorf("main() failure: expected template name")
		} else {
			err = ontap.DeleteRepoTemplate(&nodeConfig, *optTemplateName)
		}
		baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
	case "createSnapshot":
		var baseResult OperationResult = &BaseResult{}
		if nodeConfig.Compute.HostName == "" || *optSnapshotName == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName and snapshot name")
		} else {
			err = ontap.CreateSnapshot(&nodeConfig, *optSnapshotName, "")
		}
		baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
	case "deleteSnapshot":
		var baseResult OperationResult = &BaseResult{}
		if nodeConfig.Compute.HostName == "" || *optSnapshotName == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName and snapshot name")
		} else {
			err = ontap.DeleteSnapshot(&nodeConfig, *optSnapshotName)
		}
		baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
	case "restoreSnapshot":
		var baseResult OperationResult = &BaseResult{}
		if nodeConfig.Compute.HostName == "" || *optSnapshotName == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName and snapshot name")
		} else {
			err = restoreSnapshot(&nodeConfig, *optSnapshotName)
		}
		baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
	case "listImages":
		var imageResult OperationResult = &ImageResult{}
		imageResult.(*ImageResult).Images, err = ontap.GetRepoImages(&nodeConfig)
		imageResult.DumpResult(imageResult, *optDumpResult, *optEncodingFormat, err)
	case "listTemplates":
		var templateResult OperationResult = &TemplateResult{}
		templateResult.(*TemplateResult).Templates, err = ontap.GetRepoTemplates(&nodeConfig)
		templateResult.DumpResult(templateResult, *optDumpResult, *optEncodingFormat, err)
	case "listSnapshots":
		var snapshotResult OperationResult = &SnapshotResult{}
		if nodeConfig.Compute.HostName == "" {
			err = fmt.Errorf("main() failure: expected compute.hostName")
		} else {
			snapshotResult.(*SnapshotResult).Snapshots, err = ontap.GetSnapshots(&nodeConfig)
		}
		snapshotResult.DumpResult(snapshotResult, *optDumpResult, *optEncodingFormat, err)
	case "encryptConfig":
		var baseResult OperationResult = &BaseResult{}
		if err = config.EncryptNodeConfig(&nodeConfig, passPhrase); err == nil {
			dumpNodeConfig(*optDumpResult, &nodeConfig, *optEncodingFormat)
		} else {
			baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
		}
	case "decryptConfig":
		var baseResult OperationResult = &BaseResult{}
		if err = config.DecryptNodeConfig(&nodeConfig, passPhrase); err == nil {
			dumpNodeConfig("STDOUT", &nodeConfig, *optEncodingFormat)
		} else {
			baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
		}
	case "encryptString":
		var baseResult OperationResult = &BaseResult{}
		var encrypted string
		if encrypted, err = encryptString(*optSourceString, passPhrase); err == nil {
			fmt.Println(encrypted)
		} else {
			baseResult.DumpResult(baseResult, *optDumpResult, *optEncodingFormat, err)
		}
	default:
		usage()
	}
}
