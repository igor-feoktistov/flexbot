package ontap

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"bytes"
	"sync"
	"path/filepath"
	"text/template"

	"flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
	"github.com/igor-feoktistov/go-ontap-sdk/util"
	"github.com/kdomanski/iso9660"
)

func CreateSeedStorage(nodeConfig *config.NodeConfig) (err error) {
	var fileReader io.Reader
	var file *os.File
	var b []byte
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
		if strings.HasPrefix(nodeConfig.Storage.SeedLun.SeedTemplate.Location, "file://") {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location[7:])
		} else {
			file, err = os.Open(nodeConfig.Storage.SeedLun.SeedTemplate.Location)
		}
		if err != nil {
			if os.IsNotExist(err) {
				// Last resort to download template from storage repository
				b, err = DownloadRepoTemplate(nodeConfig, filepath.Base(nodeConfig.Storage.SeedLun.SeedTemplate.Location))
			}
		} else {
			fileReader = file
			defer file.Close()
		}
		if err != nil {
			err = fmt.Errorf("CreateSeedStorage: failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	}
	if len(b) == 0 {
		if b, err = ioutil.ReadAll(fileReader); err != nil {
			err = fmt.Errorf("CreateSeedStorage: failure to read cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			return
		}
	}
	var isoWriter *iso9660.ImageWriter
	if isoWriter, err = iso9660.NewWriter(); err != nil {
		err = fmt.Errorf("CreateSeedStorage: failed to create ISO writer: %v", err)
		return
	}
	defer isoWriter.Cleanup()
	for _, cloudInitData := range []string{"meta-data", "network-config", "user-data"} {
		var cloudInitBuf bytes.Buffer
		var t *template.Template
		if t, err = template.New(cloudInitData).Parse(string(b)); err != nil {
			err = fmt.Errorf("CreateSeedStorage: failure to parse cloud-init template: %s", err)
			return
		}
		if err = t.Execute(&cloudInitBuf, nodeConfig); err != nil {
			err = fmt.Errorf("CreateSeedStorage: template failure for %s: %s", cloudInitData, err)
			return
		}
		cloudInitReader := strings.NewReader(cloudInitBuf.String())
		if err = isoWriter.AddFile(cloudInitReader, cloudInitData); err != nil {
			err = fmt.Errorf("CreateSeedStorage: failed to add %s file to ISO: %v", cloudInitData, err)
			return
		}
	}
	wabuf := newWriteAtBuffer(nil)
	if err = isoWriter.WriteTo(wabuf, "cidata"); err != nil {
		err = fmt.Errorf("CreateSeedStorage: failed to write ISO image: %v", err)
		return
	}
	isoReader := bytes.NewReader(wabuf.Bytes())
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
		if _, _, err = c.FileDeleteFileAPI("/vol/" + nodeConfig.Storage.VolumeName + "/seed"); err != nil {
			err = fmt.Errorf("CreateSeedStorage: FileDeleteFileAPI() failure: %s", err)
		}
	}
	if _, err = util.UploadFileAPI(c, nodeConfig.Storage.VolumeName, "/seed", isoReader); err != nil {
		err = fmt.Errorf("CreateSeedStorage: UploadFileAPI() failure: %s", err)
		return
	}
	seedLunCreateOptions := &ontap.LunCreateFromFileOptions{
		Comment:  nodeConfig.Storage.SeedLun.SeedTemplate.Location,
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
			templateExists := false
			if os.IsNotExist(err) {
				templateExists, err = IsRepoTemplateExist(nodeConfig, nodeConfig.Storage.SeedLun.SeedTemplate.Location)
			}
			if !templateExists {
				err = fmt.Errorf("CreateSeedStoragePreflight: failure to open cloud-init template %s: %s", nodeConfig.Storage.SeedLun.SeedTemplate.Location, err)
			}
		} else {
			file.Close()
		}
	}
	return
}

// a stripped down version of the WriteAtBuffer from
// https://github.com/aws/aws-sdk-go/blob/master/aws/types.go and
// https://github.com/LINBIT/virter/blob/master/internal/virter/writeatbuffer.go

type writeAtBuffer struct {
	buf []byte
	m   sync.Mutex
}

func newWriteAtBuffer(buf []byte) *writeAtBuffer {
	return &writeAtBuffer{buf: buf}
}

func (b *writeAtBuffer) WriteAt(p []byte, pos int64) (n int, err error) {
	pLen := len(p)
	expLen := pos + int64(pLen)
	b.m.Lock()
	defer b.m.Unlock()
	if int64(len(b.buf)) < expLen {
		if int64(cap(b.buf)) < expLen {
			newBuf := make([]byte, expLen)
			copy(newBuf, b.buf)
			b.buf = newBuf
		}
		b.buf = b.buf[:expLen]
	}
	copy(b.buf[pos:], p)
	return pLen, nil
}

func (b *writeAtBuffer) Bytes() []byte {
	b.m.Lock()
	defer b.m.Unlock()
	return b.buf
}
