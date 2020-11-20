package ontap

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"encoding/hex"
	"time"

	"flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
	"github.com/igor-feoktistov/go-ontap-sdk/util"
)

const (
	imageRepoVolSize = 64
	templateRepoVolSize = 1
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
		Comment:  imageName,
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

func CreateRepoTemplate(nodeConfig *config.NodeConfig, templateName string, templatePath string) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("CreateRepoTemplate: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("CreateRepoTemplate: VolumeExists() failure: %s", err)
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
			err = fmt.Errorf("CreateRepoTemplate: VserverShowAggrGetAPI() failure: %s", err)
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
				err = fmt.Errorf("CreateRepoTemplate: no aggregates found for vserver %s", nodeConfig.Storage.SvmName)
				return
			}
		}
		// Create export policy with the same name as volume
		if _, _, err = c.ExportPolicyCreateAPI(nodeConfig.Storage.TemplateRepoName, false); err != nil {
			err = fmt.Errorf("CreateRepoTemplate: ExportPolicyCreateAPI() failure: %s", err)
			return
		}
		// Create template repository volume
		volOptions := &ontap.VolumeCreateOptions{
			VolumeType:              "rw",
			Volume:                  nodeConfig.Storage.TemplateRepoName,
			JunctionPath:            "/" + nodeConfig.Storage.TemplateRepoName,
			UnixPermissions:         "0755",
			Size:                    strconv.Itoa(templateRepoVolSize) + "g",
			ExportPolicy:            nodeConfig.Storage.TemplateRepoName,
			ContainingAggregateName: aggregateName,
		}
		if _, _, err = c.VolumeCreateAPI(volOptions); err != nil {
			err = fmt.Errorf("CreateRepoTemplate: VolumeCreateAPI() failure: %s", err)
			return
		}
		time.Sleep(10 * time.Second)
	}
	filePath := "/vol/" + nodeConfig.Storage.TemplateRepoName + "/cloud-init/" + templateName
	var fileReader io.Reader
	var fileExists bool
	if fileExists, err = util.FileExists(c, filePath); err != nil {
		err = fmt.Errorf("CreateRepoImage: FileExists() failure: %s", err)
		return
	}
	if fileExists {
		if _, _, err = c.FileTruncateFileAPI(filePath, 0); err != nil {
			err = fmt.Errorf("CreateRepoTemplate: FileTruncateFileAPI() failure: %s", err)
			return
		}
	}
	if strings.HasPrefix(templatePath, "http://") || strings.HasPrefix(templatePath, "https://") {
		var httpResponse *http.Response
		if httpResponse, err = http.Get(templatePath); err == nil {
			fileReader = httpResponse.Body
			defer httpResponse.Body.Close()
		} else {
			err = fmt.Errorf("CreateRepoTemplate: failure to open file %s: %s", templatePath, err)
			return
		}
	} else {
		var file *os.File
		if strings.HasPrefix(templatePath, "file://") {
			file, err = os.Open(templatePath[7:])
		} else {
			file, err = os.Open(templatePath)
		}
		if err != nil {
			err = fmt.Errorf("CreateRepoTemplate: failure to open file %s: %s", templatePath, err)
			return
		}
		fileReader = file
		defer file.Close()
	}
	if _, err = util.UploadFileAPI(c, nodeConfig.Storage.TemplateRepoName, "/cloud-init/" + templateName, fileReader); err != nil {
		err = fmt.Errorf("CreateRepoTemplate: UploadFileAPI() failure: %s", err)
	}
	return
}

func GetRepoTemplates(nodeConfig *config.NodeConfig) (templatesList []string, err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("GetRepoTemplates: CreateCdotClient() failure: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("GetRepoTemplates: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("GetRepoTemplates: repo volume %s does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	listDirOptions := &ontap.FileListDirectoryOptions {
		    MaxRecords: 1024,
		    Path: "/vol/" + nodeConfig.Storage.TemplateRepoName + "/cloud-init",
	}
	var listDirResponse []*ontap.FileListDirectoryResponse
	if listDirResponse, err = c.FileListDirectoryIterAPI(listDirOptions); err != nil {
		err = fmt.Errorf("GetRepoTemplates: FileListDirectoryIterAPI() failure: %s", err)
		return
	}
	for _, response := range listDirResponse {
		for _, fileAttr := range response.Results.AttributesList.FileAttributes {
			if !strings.HasPrefix(fileAttr.Name, ".") {
				templatesList = append(templatesList, fileAttr.Name)
			}
		}
	}
	return
}

func DeleteRepoTemplate(nodeConfig *config.NodeConfig, templateName string) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DeleteRepoTemplate: repo volume %s does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	var fileExists bool
	if fileExists, err = util.FileExists(c, "/vol/" + nodeConfig.Storage.TemplateRepoName + "/cloud-init/" + templateName); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate: FileExists() failure: %s", err)
		return
	}
	if fileExists {
		if _, _, err = c.FileDeleteFileAPI("/vol/" + nodeConfig.Storage.TemplateRepoName + "/cloud-init/" + templateName); err != nil {
			err = fmt.Errorf("DeleteRepoTemplate: FileDeleteFileAPI() failure: %s", err)
		}
	}
	return
}

func IsRepoTemplateExist(nodeConfig *config.NodeConfig, templateName string) (templateExists bool, err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DeleteRepoTemplate: repo volume %s does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	if templateExists, err = util.FileExists(c, "/vol/" + nodeConfig.Storage.TemplateRepoName + "/cloud-init/" + templateName); err != nil {
		err = fmt.Errorf("DeleteRepoTemplate: FileExists() failure: %s", err)
		return
	}
	return
}

func DownloadRepoTemplate(nodeConfig *config.NodeConfig, templateName string) (templateContent []byte, err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate: %s", err)
		return
	}
	var volExists bool
	if volExists, err = util.VolumeExists(c, nodeConfig.Storage.TemplateRepoName); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate: VolumeExists() failure: %s", err)
		return
	}
	if !volExists {
		err = fmt.Errorf("DownloadRepoTemplate: repo volume %s does not exist", nodeConfig.Storage.TemplateRepoName)
		return
	}
	filePath := "/vol/" + nodeConfig.Storage.TemplateRepoName + "/cloud-init/" + templateName
	var fileExists bool
	if fileExists, err = util.FileExists(c, filePath); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate: FileExists() failure: %s", err)
		return
	}
	if !fileExists {
		err = fmt.Errorf("DownloadRepoTemplate: template %s not found", nodeConfig.Storage.TemplateRepoName)
		return
	}
	var fileInfoResponse *ontap.FileGetFileInfoResponse
	if fileInfoResponse, _, err = c.FileGetFileInfoAPI(filePath); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate: FileGetFileInfoAPI() failure: %s", err)
		return
	}
	readFileOptions := &ontap.FileReadFileOptions {
		Path: filePath,
		Offset: 0,
		Length: fileInfoResponse.Results.FileInfo.FileSize,
	}
	var readFileResponse *ontap.FileReadFileResponse
	if readFileResponse, _, err = c.FileReadFileAPI(readFileOptions); err != nil {
		err = fmt.Errorf("DownloadRepoTemplate: FileReadFileAPI() failure: %s", err)
		return
	}
	bytesEncoded := []byte(readFileResponse.Results.Data)
	templateContent = make([]byte, hex.DecodedLen(len(bytesEncoded)))
	hex.Decode(templateContent, bytesEncoded)
	return
}
