package ontap

import (
	"fmt"

	"flexbot/pkg/config"
	"github.com/igor-feoktistov/go-ontap-sdk/ontap"
)

func SnapshotExists(nodeConfig *config.NodeConfig, snapshotName string) (exists bool, err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	options := &ontap.SnapshotListInfoOptions {
		Volume: nodeConfig.Storage.VolumeName,
	}
	var response *ontap.SnapshotListInfoResponse
	if response, _, err = c.SnapshotListInfoAPI(options); err != nil {
		err = fmt.Errorf("GetSnapshots: SnapshotListInfoAPI() failure: %s", err)
		return
	}
	for _, snapshot := range response.Results.Snapshots {
		if snapshotName == snapshot.Name {
			exists = true
			break
		}
	}
	return
}

func GetSnapshots(nodeConfig *config.NodeConfig) (snapshotList []string, err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	options := &ontap.SnapshotListInfoOptions {
		Volume: nodeConfig.Storage.VolumeName,
	}
	var response *ontap.SnapshotListInfoResponse
	if response, _, err = c.SnapshotListInfoAPI(options); err != nil {
		err = fmt.Errorf("GetSnapshots: SnapshotListInfoAPI() failure: %s", err)
		return
	}
	nodeConfig.Storage.Snapshots = []string{}
	for _, snapshot := range response.Results.Snapshots {
		snapshotList = append(snapshotList, snapshot.Name)
		nodeConfig.Storage.Snapshots = append(nodeConfig.Storage.Snapshots, snapshot.Name)
	}
	return
}

func CreateSnapshot(nodeConfig *config.NodeConfig, snapshotName string, snapshotComment string) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	options := &ontap.SnapshotCreateOptions {
		Volume: nodeConfig.Storage.VolumeName,
		Snapshot: snapshotName,
		Comment: snapshotComment,
	}
	if _, _, err = c.SnapshotCreateAPI(options); err != nil {
		err = fmt.Errorf("CreateSnapshot: SnapshotCreateAPI() failure: %s", err)
	}
	return
}

func DeleteSnapshot(nodeConfig *config.NodeConfig, snapshotName string) (err error) {
	var c *ontap.Client
	var response *ontap.SingleResultResponse
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	options := &ontap.SnapshotDeleteOptions {
		Volume: nodeConfig.Storage.VolumeName,
		Snapshot: snapshotName,
	}
	if response, _, err = c.SnapshotDeleteAPI(options); err != nil {
		if response.Results.ErrorNo != ontap.ENTRYDOESNOTEXIST {
			err = fmt.Errorf("DeleteSnapshot: SnapshotDeleteAPI() failure: %s", err)
		} else {
			err = nil
		}
	}
	return
}

func RestoreSnapshot(nodeConfig *config.NodeConfig, snapshotName string) (err error) {
	var c *ontap.Client
	if c, err = CreateCdotClient(nodeConfig); err != nil {
		return
	}
	options := &ontap.SnapshotRestoreVolumeOptions {
		PreserveLunIds: false,
		Volume: nodeConfig.Storage.VolumeName,
		Snapshot: snapshotName,
	}
	if _, _, err = c.SnapshotRestoreVolumeAPI(options); err != nil {
		err = fmt.Errorf("RestoreSnapshot: SnapshotRestoreVolumeAPI() failure: %s", err)
	}
	return
}
