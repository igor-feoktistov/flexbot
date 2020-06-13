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
	version = "1.2.0"
)

type NodeResult struct {
	Status       string             `yaml:"status" json:"status"`
	ErrorMessage string             `yaml:"errorMessage,omitempty" json:"errorMessage,omitempty"`
	Node         *config.NodeConfig `yaml:"server,omitempty" json:"server,omitempty"`
}

type ImageResult struct {
	Status       string   `yaml:"status" json:"status"`
	ErrorMessage string   `yaml:"errorMessage,omitempty" json:"errorMessage,omitempty"`
	Images       []string `yaml:"images,omitempty" json:"images,omitempty"`
}

type TemplateResult struct {
	Status       string   `yaml:"status" json:"status"`
	ErrorMessage string   `yaml:"errorMessage,omitempty" json:"errorMessage,omitempty"`
	Templates    []string `yaml:"templates,omitempty" json:"templates,omitempty"`
}

func Usage() {
	goOS := runtime.GOOS
	goARCH := runtime.GOARCH
	fmt.Printf("flexbot version %s %s/%s\n\n", version, goOS, goARCH)
	flag.Usage()
	fmt.Println("")
	fmt.Printf("flexbot --config=<config file path> --op=provisionServer --host=<host node name> --image=<image name> --templatePath=<cloud-init template path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=stopServer --host=<host node name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=startServer --host=<host node name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deprovisionServer --host=<host node name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=decryptConfig [--passphrase=<password phrase>]\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=encryptConfig [--passphrase=<password phrase>]\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=uploadImage --image=<image name> --imagePath=<image path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deleteImage --image=<image name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=listImages\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=uploadTemplate --template=<template name> --templatePath=<template path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=downloadTemplate --template=<template name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deleteTemplate --template=<template name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=listTemplates\n\n")
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

func ProvisionServer(nodeConfig *config.NodeConfig) (err error) {
	var provider ipam.IpamProvider
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("IPAM:Provider \"%s\" is not implemented", nodeConfig.Ipam.Provider)
		return
	}
	if err = provider.Allocate(nodeConfig); err != nil {
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

func DiscoverServer(nodeConfig *config.NodeConfig) (serverExists bool, err error) {
	if serverExists, err = ucsm.DiscoverServer(nodeConfig); err != nil {
		return
	}
	if serverExists {
		var provider ipam.IpamProvider
		switch nodeConfig.Ipam.Provider {
		case "Infoblox":
			provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
		case "Internal":
			provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
		default:
			err = fmt.Errorf("IPAM:Provider \"%s\" is not implemented", nodeConfig.Ipam.Provider)
			return
		}
		if err = provider.Discover(nodeConfig); err != nil {
			return
		}
	}
	return
}

func ProvisionServerPreflight(nodeConfig *config.NodeConfig) (err error) {
	var provider ipam.IpamProvider
	var stepErr error
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("IPAM:Provider \"%s\" is not implemented", nodeConfig.Ipam.Provider)
	}
	if stepErr = provider.AllocatePreflight(nodeConfig); stepErr != nil {
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

func DeprovisionServer(nodeConfig *config.NodeConfig) (err error) {
	var provider ipam.IpamProvider
	var stepErr error
	var powerState string

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
	switch nodeConfig.Ipam.Provider {
	case "Infoblox":
		provider = ipam.NewInfobloxProvider(&nodeConfig.Ipam)
	case "Internal":
		provider = ipam.NewInternalProvider(&nodeConfig.Ipam)
	default:
		err = fmt.Errorf("IPAM:Provider \"%s\" is not implemnted", nodeConfig.Ipam.Provider)
		return
	}
	if stepErr = ontap.DeleteBootStorage(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	if stepErr = provider.Release(nodeConfig); stepErr != nil {
		if err == nil {
			err = stepErr
		} else {
			err = fmt.Errorf("%s\n%s", err, stepErr)
		}
	}
	return
}

func StopServer(nodeConfig *config.NodeConfig) (err error) {
	err = ucsm.StopServer(nodeConfig)
	return
}

func StartServer(nodeConfig *config.NodeConfig) (err error) {
	err = ucsm.StartServer(nodeConfig)
	return
}

func UploadImage(nodeConfig *config.NodeConfig, imageName string, imagePath string) (err error) {
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
	if err != nil {
		fmt.Printf("\n%s\n", err)
	}
	return
}

func UploadTemplate(nodeConfig *config.NodeConfig, templateName string, templatePath string) (err error) {
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
	if err != nil {
		fmt.Printf("\n%s\n", err)
	}
	return
}

func DeleteImage(nodeConfig *config.NodeConfig, imageName string) (err error) {
	fmt.Printf("Deleting image..")
	if err = ontap.DeleteRepoImage(nodeConfig, imageName); err == nil {
		fmt.Println("succes")
	} else {
		fmt.Println("failure")
		fmt.Printf("\n%s\n", err)
	}
	return
}

func DeleteTemplate(nodeConfig *config.NodeConfig, templateName string) (err error) {
	fmt.Printf("Deleting template..")
	if err = ontap.DeleteRepoTemplate(nodeConfig, templateName); err == nil {
		fmt.Println("succes")
	} else {
		fmt.Println("failure")
		fmt.Printf("\n%s\n", err)
	}
	return
}

func DownloadTemplate(nodeConfig *config.NodeConfig, templateName string) (err error) {
	var templateContent []byte
	if templateContent, err = ontap.DownloadRepoTemplate(nodeConfig, templateName); err != nil {
		panic("Failure to download template: " + err.Error())
	}
	fmt.Print(string(templateContent))
	return
}

func DumpNodeResult(resultDest string, nodeConfig *config.NodeConfig, format string, resultErr error) {
	var b []byte
	var nodeResult NodeResult
	var err error
	nodeResult.Node = nodeConfig
	nodeResult.Node.Ipam.IbCredentials = config.InfobloxCredentials{}
	nodeResult.Node.Storage.CdotCredentials = config.Credentials{}
	nodeResult.Node.Compute.UcsmCredentials = config.Credentials{}
	nodeResult.Node.CloudArgs = map[string]string{}
	if resultErr == nil {
		nodeResult.Status = "success"
	} else {
		nodeResult.Status = "failure"
		nodeResult.ErrorMessage = resultErr.Error()
	}
	if format == "yaml" {
		b, err = yaml.Marshal(nodeResult)
	} else {
		b, err = json.Marshal(nodeResult)
	}
	if err != nil {
		panic("Failure to decode node result: " + err.Error())
	} else {
		if resultDest == "STDOUT" {
			fmt.Print(string(b))
		} else {
			if err = ioutil.WriteFile(resultDest, b, 0644); err != nil {
				panic("Failure to write node result: " + err.Error())
			}
		}
	}
}

func DumpImageResult(resultDest string, images []string, format string, resultErr error) {
	var b []byte
	var err error
	var imageResult ImageResult
	imageResult.Images = images
	if resultErr == nil {
		imageResult.Status = "success"
	} else {
		imageResult.Status = "failure"
		imageResult.ErrorMessage = resultErr.Error()
	}
	if format == "yaml" {
		b, err = yaml.Marshal(imageResult)
	} else {
		b, err = json.Marshal(imageResult)
	}
	if err != nil {
		panic("Failure to decode image result: " + err.Error())
	} else {
		if resultDest == "STDOUT" {
			fmt.Print(string(b))
		} else {
			if err = ioutil.WriteFile(resultDest, b, 0644); err != nil {
				panic("Failure to write image result: " + err.Error())
			}
		}
	}
}

func DumpTemplateResult(resultDest string, templates []string, format string, resultErr error) {
	var b []byte
	var err error
	var templateResult TemplateResult
	templateResult.Templates = templates
	if resultErr == nil {
		templateResult.Status = "success"
	} else {
		templateResult.Status = "failure"
		templateResult.ErrorMessage = resultErr.Error()
	}
	if format == "yaml" {
		b, err = yaml.Marshal(templateResult)
	} else {
		b, err = json.Marshal(templateResult)
	}
	if err != nil {
		panic("Failure to decode template result: " + err.Error())
	} else {
		if resultDest == "STDOUT" {
			fmt.Print(string(b))
		} else {
			if err = ioutil.WriteFile(resultDest, b, 0644); err != nil {
				panic("Failure to write template result: " + err.Error())
			}
		}
	}
}

func DumpNodeConfig(configDest string, nodeConfig *config.NodeConfig, format string) {
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

func EncryptString(srcString string, passPhrase string) (encrypted string, err error) {
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
	optPassPhrase := flag.String("passphrase", "", "passphrase to encrypt/decrypt passwords in configuration (default is machineid)")
	optSourceString := flag.String("sourceString", "", "source string to encrypt")
	optNodeConfig := flag.String("config", "STDIN", "a path to configuration file, STDIN, or argument value in JSON")
	optOp := flag.String("op", "", "operation: \n\tprovisionServer\n\tdeprovisionServer\n\tstopServer\n\tstartServer\n\tuploadImage\n\tlistImages\n\tencryptConfig\n\tdecryptConfig\n\tencryptString")
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
			return
		}
	} else {
		passPhrase = *optPassPhrase
	}
	if *optOp == "provisionServer" ||
		*optOp == "deprovisionServer" ||
		*optOp == "stopServer" ||
		*optOp == "startServer" ||
		*optOp == "uploadImage" ||
		*optOp == "uploadTemplate" ||
		*optOp == "downloadTemplate" ||
		*optOp == "listImages" ||
		*optOp == "listTemplates" ||
		*optOp == "deleteImage" ||
		*optOp == "deleteTemplate" ||
		*optOp == "encryptConfig" ||
		*optOp == "decryptConfig" {
		if err = config.ParseNodeConfig(*optNodeConfig, &nodeConfig); err != nil {
			panic(err.Error())
		}
	}
	switch *optOp {
	case "provisionServer":
		if err = config.SetDefaults(&nodeConfig, *optHostName, *optImageName, *optTemplateName, passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
		} else {
			if nodeConfig.Compute.HostName == "" || nodeConfig.Storage.BootLun.OsImage.Name == "" || nodeConfig.Storage.SeedLun.SeedTemplate.Location == "" {
				err = fmt.Errorf("SetDefaults() failure: expected compute.hostName, storage.bootLun.osImage.name, and storage.seedLun.seedTemplate.location")
			} else {
				var serverExists bool
				if serverExists, err = DiscoverServer(&nodeConfig); err == nil {
					if serverExists == false {
						if err = ProvisionServerPreflight(&nodeConfig); err == nil {
							if err = ProvisionServer(&nodeConfig); err != nil {
								DeprovisionServer(&nodeConfig)
							}
						}
					}
				}
			}
		}
		DumpNodeResult(*optDumpResult, &nodeConfig, *optEncodingFormat, err)
	case "deprovisionServer":
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
		} else {
			if nodeConfig.Compute.HostName == "" {
				err = fmt.Errorf("SetDefaults() failure: expected compute.hostName")
			} else {
				err = DeprovisionServer(&nodeConfig)
			}
		}
		DumpNodeResult(*optDumpResult, &nodeConfig, *optEncodingFormat, err)
	case "stopServer":
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
		} else {
			if nodeConfig.Compute.HostName == "" {
				err = fmt.Errorf("SetDefaults() failure: expected compute.hostName")
			} else {
				err = StopServer(&nodeConfig)
			}
		}
		DumpNodeResult(*optDumpResult, &nodeConfig, *optEncodingFormat, err)
	case "startServer":
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
		} else {
			if nodeConfig.Compute.HostName == "" {
				err = fmt.Errorf("SetDefaults() failure: expected compute.hostName")
			} else {
				err = StartServer(&nodeConfig)
			}
		}
		DumpNodeResult(*optDumpResult, &nodeConfig, *optEncodingFormat, err)
	case "uploadImage":
		if *optImageName == "" || *optImagePath == "" {
			Usage()
			return
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			UploadImage(&nodeConfig, *optImageName, *optImagePath)
		}
	case "uploadTemplate":
		if *optTemplateName == "" || *optTemplatePath == "" {
			Usage()
			return
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			UploadTemplate(&nodeConfig, *optTemplateName, *optTemplatePath)
		}
	case "downloadTemplate":
		if *optTemplateName == "" {
			Usage()
			return
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			DownloadTemplate(&nodeConfig, *optTemplateName)
		}
	case "deleteImage":
		if *optImageName == "" {
			Usage()
			return
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			DeleteImage(&nodeConfig, *optImageName)
		}
	case "deleteTemplate":
		if *optTemplateName == "" {
			Usage()
			return
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			DeleteTemplate(&nodeConfig, *optTemplateName)
		}
	case "listImages":
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			var images []string
			images, err = ontap.GetRepoImages(&nodeConfig)
			DumpImageResult("STDOUT", images, *optEncodingFormat, err)
		}
	case "listTemplates":
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
			fmt.Printf("%s\n\n", err.Error())
		} else {
			var templates []string
			templates, err = ontap.GetRepoTemplates(&nodeConfig)
			DumpTemplateResult("STDOUT", templates, *optEncodingFormat, err)
		}
	case "encryptConfig":
		if err = config.EncryptNodeConfig(&nodeConfig, passPhrase); err == nil {
			DumpNodeConfig("STDOUT", &nodeConfig, *optEncodingFormat)
		} else {
			fmt.Printf("%s\n\n", err.Error())
		}
	case "decryptConfig":
		if err = config.DecryptNodeConfig(&nodeConfig, passPhrase); err == nil {
			DumpNodeConfig("STDOUT", &nodeConfig, *optEncodingFormat)
		} else {
			fmt.Printf("%s\n\n", err.Error())
		}
	case "encryptString":
		var encrypted string
		if encrypted, err = EncryptString(*optSourceString, passPhrase); err == nil {
			fmt.Println(encrypted)
		} else {
			fmt.Printf("%s\n\n", err.Error())
			Usage()
		}
	default:
		Usage()
	}
}
