package main

import (
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
	"github.com/denisbrodbeck/machineid"
	"gopkg.in/yaml.v3"
)

const (
	version = "1.1.6"
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
	fmt.Printf("flexbot --config=<config file path> --op=decryptConfig\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=encryptConfig\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=uploadImage --image=<image name> --imagePath=<image path>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=deleteImage --image=<image name>\n\n")
	fmt.Printf("flexbot --config=<config file path> --op=listImages\n\n")
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

func main() {
	var nodeConfig config.NodeConfig
	var err error
	var passPhrase string
	optHostName := flag.String("host", "", "compute node name")
	optImageName := flag.String("image", "", "boot image name")
	optImagePath := flag.String("imagePath", "", "a path to boot image (prefix can be either file:// or http(s)://)")
	optTemplate := flag.String("template", "", "a path to cloud-init template (prefix can be either file:// or http(s)://)")
	optPassPhrase := flag.String("passphrase", "", "passphrase to encrypt/decrypt passwords in configuration (default is machineid)")
	optNodeConfig := flag.String("config", "STDIN", "a path to configuration file, STDIN, or argument value in JSON")
	optOp := flag.String("op", "", "operation: \n\tprovisionServer\n\tdeprovisionServer\n\tstopServer\n\tstartServer\n\tuploadImage\n\tlistImages\n\tencryptConfig\n\tdecryptConfig")
	optDumpResult := flag.String("dumpResult", "STDOUT", "dump result: file path or STDOUT")
	optEncodingFormat := flag.String("encodingFormat", "yaml", "supported encoding formats: json, yaml")
	optVersion := flag.Bool("version", false, "flexbot version")
	flag.Parse()
	if *optVersion {
		goOS := runtime.GOOS
		goARCH := runtime.GOARCH
		fmt.Printf("flexbot version %s %s/%s\n", version, goOS, goARCH)
		return
	} else {
		if *optOp == "" {
			Usage()
			return
		}
	}
	if *optPassPhrase == "" {
		if passPhrase, err = machineid.ID(); err != nil {
			return
		}
	} else {
		passPhrase = *optPassPhrase
	}
	if err = config.ParseNodeConfig(*optNodeConfig, &nodeConfig); err != nil {
		panic(err.Error())
	}
	switch *optOp {
	case "provisionServer":
		if err = config.SetDefaults(&nodeConfig, *optHostName, *optImageName, *optTemplate, passPhrase); err != nil {
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
		} else {
			UploadImage(&nodeConfig, *optImageName, *optImagePath)
		}
	case "deleteImage":
		if *optImageName == "" {
			Usage()
			return
		}
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
		} else {
			DeleteImage(&nodeConfig, *optImageName)
		}
	case "listImages":
		if err = config.SetDefaults(&nodeConfig, *optHostName, "", "", passPhrase); err != nil {
			err = fmt.Errorf("SetDefaults() failure: %s", err)
		} else {
			var images []string
			images, err = ontap.GetRepoImages(&nodeConfig)
			DumpImageResult("STDOUT", images, *optEncodingFormat, err)
		}
	case "encryptConfig":
		if err = config.EncryptNodeConfig(&nodeConfig, passPhrase); err == nil {
			DumpNodeConfig("STDOUT", &nodeConfig, *optEncodingFormat)
		}
	case "decryptConfig":
		if err = config.DecryptNodeConfig(&nodeConfig, passPhrase); err == nil {
			DumpNodeConfig("STDOUT", &nodeConfig, *optEncodingFormat)
		}
	default:
		Usage()
	}
}
