package ontap

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"

	"flexbot/pkg/config"
	diskfs "flexbot/pkg/diskfs"
	"flexbot/pkg/diskfs/disk"
	"flexbot/pkg/diskfs/filesystem"
	"flexbot/pkg/diskfs/filesystem/iso9660"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
	"github.com/igor-feoktistov/go-ontap-sdk/util"
)

func CreateSeedStorage(nodeConfig *config.NodeConfig) (err error) {
	var diskSize int64 = 1 * 1024 * 1024
	var isoDisk *disk.Disk
	seedImage := nodeConfig.Compute.HostName + "-seed.iso"
	var fileReader io.Reader
	if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(nodeConfig.Storage.SeedLun.SeedTemplate.Location); err == nil {
			fileReader = httpResponse.Body
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateSeedStorage: failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	} else {
		var file *os.File
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			err = fmt.Errorf("CreateSeedStorage: failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
		fileReader = file
		defer file.Close()
	}
	var b []byte
	if b, err = ioutil.ReadAll(fileReader); err != nil {
		err = fmt.Errorf("CreateSeedStorage: failure to read cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
		return
	}
	os.Remove(seedImage)
	if isoDisk, err = diskfs.Create(seedImage, diskSize, diskfs.Raw); err != nil {
		err = fmt.Errorf("CreateSeedStorage: failure to create ISO image: %s", err)
		return
	}
	isoDisk.LogicalBlocksize = 2048
	fsSpec := disk.FilesystemSpec{Partition: 0, FSType: filesystem.TypeISO9660, VolumeLabel: "cidata"}
	var fs filesystem.FileSystem
	if fs, err = isoDisk.CreateFilesystem(fsSpec); err != nil {
		err = fmt.Errorf("CreateSeedStorage: failure to crete ISO9660 filesystem: %s", err)
		return
	}
	for _, cloudInitData := range []string{"meta-data", "network-config", "user-data"} {
		var cloudInitFile filesystem.File
		var t *template.Template
		if t, err = template.New(cloudInitData).Parse(string(b)); err != nil {
			err = fmt.Errorf("CreateSeedStorage: failure to parse cloud-init template: %s", err)
			return
		}
		if cloudInitFile, err = fs.OpenFile(cloudInitData, os.O_CREATE|os.O_RDWR); err != nil {
			err = fmt.Errorf("CreateSeedStorage: failure to open file %s: %s", cloudInitData, err)
			return
		}
		if err = t.Execute(cloudInitFile, nodeConfig); err != nil {
			err = fmt.Errorf("CreateSeedStorage: template failure for %s: %s", cloudInitData, err)
			return
		}
	}
	if iso, ok := fs.(*iso9660.FileSystem); ok {
		if err = iso.Finalize(iso9660.FinalizeOptions{}); err != nil {
			err = fmt.Errorf("CreateSeedStorage: iso.Finalize() failure: %s", err)
			return
		}
	} else {
		err = fmt.Errorf("CreateSeedStorage: not an iso9660 filesystem")
		return
	}
	defer os.Remove(seedImage)
	var file *os.File
	if file, err = os.Open(seedImage); err == nil {
		fileReader = file
		defer file.Close()
	} else {
		err = fmt.Errorf("CreateSeedStorage: cloud not open file %s: %s", seedImage, err)
		return
	}
	var c *ontap.Client
	var response *ontap.SingleResultResponse
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateSeedStorage: CreateCdotClient() failure: %s", err)
		return
	}
	var fileExists bool
	if fileExists, err = util.FileExists(c, "/vol/"+nodeConfig.Storage.VolumeName+"/seed"); err != nil {
		err = fmt.Errorf("CreateSeedStorage: FileExists() failure: %s", err)
		return
	}
	if fileExists {
		var lunExists bool
		if lunExists, err = util.LunExists(c, "/vol/"+nodeConfig.Storage.VolumeName+"/"+nodeConfig.Storage.SeedLun.Name); err != nil {
			err = fmt.Errorf("CreateSeedStorage: LunExists() failure: %s", err)
			return
		} else {
			if lunExists {
				seedLunUnmapOptions := &ontap.LunUnmapOptions{
					InitiatorGroup: nodeConfig.Storage.IgroupName,
					Path:           "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name,
				}
				_, _, _ = c.LunUnmapAPI(seedLunUnmapOptions)
				seedLunDestroyOptions := &ontap.LunDestroyOptions{
					Path: "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name,
				}
				if response, _, err = c.LunDestroyAPI(seedLunDestroyOptions); err != nil {
					if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
						err = fmt.Errorf("CreateSeedStorage: LunDestroyAPI() failure: %s", err)
						return
					}
				}
			}
		}
		if _, _, err = c.FileTruncateFileAPI("/vol/"+nodeConfig.Storage.VolumeName+"/seed", 0); err != nil {
			err = fmt.Errorf("CreateSeedStorage: FileTruncateFileAPI() failure: %s", err)
			return
		}
	}
	if _, err = util.UploadFileAPI(c, nodeConfig.Storage.VolumeName, "/seed", fileReader); err != nil {
		err = fmt.Errorf("CreateSeedStorage: UploadFileAPI() failure: %s", err)
		return
	}
	seedLunCreateOptions := &ontap.LunCreateFromFileOptions{
		FileName: "/vol/" + nodeConfig.Storage.VolumeName + "/seed",
		Path:     "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name,
		OsType:   "linux",
	}
	if _, _, err = c.LunCreateFromFileAPI(seedLunCreateOptions); err != nil {
		err = fmt.Errorf("CreateSeedStorage: LunCreateFromFileAPI() failure: %s", err)
		return
	}
	seedLunMapOptions := &ontap.LunMapOptions{
		LunId:          nodeConfig.Storage.SeedLun.Id,
		InitiatorGroup: nodeConfig.Storage.IgroupName,
		Path:           "/vol/" + nodeConfig.Storage.VolumeName + "/" + nodeConfig.Storage.SeedLun.Name,
	}
	if _, _, err = c.LunMapAPI(seedLunMapOptions); err != nil {
		err = fmt.Errorf("CreateSeedStorage: LunMapAPI() failure: %s", err)
	}
	return
}

func CreateSeedStoragePreflight(nodeConfig *config.NodeConfig) (err error) {
	if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "http://") || strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(nodeConfig.Storage.SeedLun.SeedTemplate.Location); err == nil {
			httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateSeedStoragePreflight: failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
		}
	} else {
		var file *os.File
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			err = fmt.Errorf("CreateSeedStoragePreflight: failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
		} else {
			file.Close()
		}
	}
	return
}
