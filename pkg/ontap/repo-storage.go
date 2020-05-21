package ontap

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
	"github.com/igor-feoktistov/go-ontap-sdk/util"
)

const (
	imageRepoVolSize = 50
)

func CreateRepoImage(nodeConfig *config.NodeConfig, imageName string, imagePath string) (err error) {
	var c *ontap.Client
	var response *ontap.SingleResultResponse
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateRepoImage: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("CreateRepoImage: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		var aggregateName string
		var aggrResponse *ontap.VserverShowAggrGetResponse
		// Find aggregate with MAX space available
		aggrOptions := &ontap.VserverShowAggrGetOptions{
			MaxRecords: 1024,
			Vserver:    nodeConfig.Storage.SvmName,
		}
		if aggrResponse, _, err = c.VserverShowAggrGetAPI(aggrOptions); err != nil {
			err = fmt.Errorf("CreateRepoImage: VserverShowAggrGetAPI() failure: %s", err)
			return
		} else {
			if aggrResponse.Results.NumRecords > 0 {
				var maxAvailableSize int
				for _, aggr := range aggrResponse.Results.AggrAttributes {
					if aggr.AvailableSize > maxAvailableSize {
						aggregateName = aggr.AggregateName
						maxAvailableSize = aggr.AvailableSize
					}
				}
			} else {
				err = fmt.Errorf("CreateRepoImage: no aggregates found for vserver %s", nodeConfig.Storage.SvmName)
				return
			}
		}
		// Create export policy with the same name as volume
		if _, _, err = c.ExportPolicyCreateAPI(nodeConfig.Storage.ImageRepoName, false); err != nil {
			err = fmt.Errorf("CreateRepoImage: ExportPolicyCreateAPI() failure: %s", err)
			return
		}
		// Create image repository volume
		volOptions := &ontap.VolumeCreateOptions{
			VolumeType:              "rw",
			Volume:                  nodeConfig.Storage.ImageRepoName,
			JunctionPath:            "/" + nodeConfig.Storage.ImageRepoName,
			UnixPermissions:         "0755",
			Size:                    strconv.Itoa(imageRepoVolSize) + "g",
			ExportPolicy:            nodeConfig.Storage.ImageRepoName,
			ContainingAggregateName: aggregateName,
		}
		if _, _, err = c.VolumeCreateAPI(volOptions); err != nil {
			err = fmt.Errorf("CreateRepoImage: VolumeCreateAPI() failure: %s", err)
			return
		}
		time.Sleep(10 * time.Second)
	}
	var lunExists bool
	if lunExists, err = util.LunExists(c, "/vol/"+nodeConfig.Storage.ImageRepoName+"/"+imageName); err != nil {
		err = fmt.Errorf("CreateRepoImage: LunExists() failure: %s", err)
		return
	} else {
		if lunExists {
			repoLunDestroyOptions := &ontap.LunDestroyOptions{
				Path: "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName,
			}
			if response, _, err = c.LunDestroyAPI(repoLunDestroyOptions); err != nil {
				if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
					err = fmt.Errorf("DeleteBootImage: LunDestroyAPI() failure: %s", err)
					return
				}
			}
		}
	}
	var fileReader io.Reader
	var fileExists bool
	if fileExists, err = util.FileExists(c, "/vol/"+nodeConfig.Storage.ImageRepoName+"/_"+imageName); err != nil {
		err = fmt.Errorf("CreateRepoImage: FileExists() failure: %s", err)
		return
	}
	if fileExists {
		if _, _, err = c.FileTruncateFileAPI("/vol/"+nodeConfig.Storage.ImageRepoName+"/_"+imageName, 0); err != nil {
			err = fmt.Errorf("CreateRepoImage: FileTruncateFileAPI() failure: %s", err)
			return
		}
	}
	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(imagePath); err == nil {
			fileReader = httpResponse.Body
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateRepoImage: failure to open file %s: %s", imagePath, err)
			return
		}
	} else {
		var file *os.File
		if strings.HasPrefix(imagePath, "file://") {
			file, err = os.Open(imagePath[7:])
		} else {
			file, err = os.Open(imagePath)
		}
		if err != nil {
			err = fmt.Errorf("CreateRepoImage: failure to open file %s: %s", imagePath, err)
			return
		}
		fileReader = file
		defer file.Close()
	}
	if _, err = util.UploadFileNFS(c, nodeConfig.Storage.ImageRepoName, "/_"+imageName, fileReader); err != nil {
		err = fmt.Errorf("CreateRepoImage: UploadFileNFS() failure: %s", err)
		return
	}
	// Create OS image LUN from image file
	lunOptions := &ontap.LunCreateFromFileOptions{
		FileName: "/vol/" + nodeConfig.Storage.ImageRepoName + "/_" + imageName,
		Path:     "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName,
		OsType:   "linux",
	}
	if _, _, err = c.LunCreateFromFileAPI(lunOptions); err != nil {
		err = fmt.Errorf("CreateRepoImage: LunCreateFromFileAPI() failure: %s", err)
	}
	return
}

func DeleteRepoImage(nodeConfig *config.NodeConfig, imageName string) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteRepoImage: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("DeleteRepoImage: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DeleteRepoImage: repo volume %s does not exist", nodeConfig.Storage.ImageRepoName)
		return
	}
	var lunExists bool
	if lunExists, err = util.LunExists(c, "/vol/"+nodeConfig.Storage.ImageRepoName+"/"+imageName); err != nil {
		err = fmt.Errorf("DeleteRepoImage: LunExists() failure: %s", err)
		return
	}
	if lunExists {
		repoLunDestroyOptions := &ontap.LunDestroyOptions{
			Path: "/vol/" + nodeConfig.Storage.ImageRepoName + "/" + imageName,
		}
		if _, _, err = c.LunDestroyAPI(repoLunDestroyOptions); err != nil {
			err = fmt.Errorf("DeleteBootImage: LunDestroyAPI() failure: %s", err)
			return
		}
	}
	var fileExists bool
	if fileExists, err = util.FileExists(c, "/vol/"+nodeConfig.Storage.ImageRepoName+"/_"+imageName); err != nil {
		err = fmt.Errorf("DeleteRepoImage: FileExists() failure: %s", err)
		return
	}
	if fileExists {
		if _, _, err = c.FileDeleteFileAPI("/vol/" + nodeConfig.Storage.ImageRepoName + "/_" + imageName); err != nil {
			err = fmt.Errorf("DeleteRepoImage: FileDeleteFileAPI() failure: %s", err)
		}
	}
	return
}

func GetRepoImages(nodeConfig *config.NodeConfig) (imagesList []string, err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("GetRepoImages: CreateCdotClient() failure: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.ImageRepoName); err != nil {
		err = fmt.Errorf("GetRepoImages: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("GetRepoImages: repo volume %s does not exist", nodeConfig.Storage.ImageRepoName)
		return
	}
	options := &ontap.LunGetOptions{
		MaxRecords: 1024,
		Query: &ontap.LunQuery{
			LunInfo: &ontap.LunInfo{
				Volume: nodeConfig.Storage.ImageRepoName,
			},
		},
	}
	var response []*ontap.LunGetResponse
	response, err = c.LunGetIterAPI(options)
	if err != nil {
		err = fmt.Errorf("GetRepoImages: LunGetIterAPI() failure: %s", err)
	} else {
		for _, responseLun := range response {
			for _, lun := range responseLun.Results.AttributesList.LunAttributes {
				imagesList = append(imagesList, lun.Path[(strings.LastIndex(lun.Path, "/")+1):])
			}
		}
	}
	return
}
