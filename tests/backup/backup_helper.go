package tests

import (
	"context"
	"fmt"
	"github.com/portworx/torpedo/drivers"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	volsnapv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	"github.com/pborman/uuid"
	"github.com/portworx/sched-ops/k8s/batch"
	"github.com/portworx/torpedo/pkg/osutils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/hashicorp/go-version"
	"github.com/libopenstorage/stork/pkg/k8sutils"
	. "github.com/onsi/ginkgo"
	api "github.com/portworx/px-backup-api/pkg/apis/v1"
	"github.com/portworx/sched-ops/k8s/apps"
	"github.com/portworx/sched-ops/k8s/core"
	"github.com/portworx/sched-ops/k8s/operator"
	"github.com/portworx/sched-ops/task"
	"github.com/portworx/torpedo/drivers/backup"
	"github.com/portworx/torpedo/drivers/node"
	"github.com/portworx/torpedo/drivers/scheduler"
	"github.com/portworx/torpedo/drivers/scheduler/k8s"
	"github.com/portworx/torpedo/drivers/volume"
	"github.com/portworx/torpedo/pkg/log"
	. "github.com/portworx/torpedo/tests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	snapv1 "github.com/kubernetes-incubator/external-storage/snapshot/pkg/apis/crd/v1"
	storageapi "k8s.io/api/storage/v1"
)

const (
	cloudAccountDeleteTimeout                 = 30 * time.Minute
	cloudAccountDeleteRetryTime               = 30 * time.Second
	storkDeploymentName                       = "stork"
	defaultStorkDeploymentNamespace           = "kube-system"
	upgradeStorkImage                         = "TARGET_STORK_VERSION"
	latestStorkImage                          = "23.3.1"
	restoreNamePrefix                         = "tp-restore"
	destinationClusterName                    = "destination-cluster"
	appReadinessTimeout                       = 10 * time.Minute
	taskNamePrefix                            = "pxbackuptask"
	orgID                                     = "default"
	usersToBeCreated                          = "USERS_TO_CREATE"
	groupsToBeCreated                         = "GROUPS_TO_CREATE"
	maxUsersInGroup                           = "MAX_USERS_IN_GROUP"
	maxBackupsToBeCreated                     = "MAX_BACKUPS"
	errorChannelSize                          = 50
	maxWaitPeriodForBackupCompletionInMinutes = 40
	maxWaitPeriodForRestoreCompletionInMinute = 40
	maxWaitPeriodForBackupJobCancellation     = 20
	maxWaitPeriodForRestoreJobCancellation    = 20
	restoreJobCancellationRetryTime           = 30
	restoreJobProgressRetryTime               = 1
	backupJobCancellationRetryTime            = 5
	K8sNodeReadyTimeout                       = 10
	K8sNodeRetryInterval                      = 30
	globalAWSBucketPrefix                     = "global-aws"
	globalAzureBucketPrefix                   = "global-azure"
	globalGCPBucketPrefix                     = "global-gcp"
	globalNFSBucketPrefix                     = "global-nfs"
	globalAWSLockedBucketPrefix               = "global-aws-locked"
	globalAzureLockedBucketPrefix             = "global-azure-locked"
	globalGCPLockedBucketPrefix               = "global-gcp-locked"
	mongodbStatefulset                        = "pxc-backup-mongodb"
	pxBackupDeployment                        = "px-backup"
	backupDeleteTimeout                       = 60 * time.Minute
	backupDeleteRetryTime                     = 30 * time.Second
	backupLocationDeleteTimeout               = 60 * time.Minute
	backupLocationDeleteRetryTime             = 30 * time.Second
	rebootNodeTimeout                         = 1 * time.Minute
	rebootNodeTimeBeforeRetry                 = 5 * time.Second
	latestPxBackupVersion                     = "2.4.0"
	defaultPxBackupHelmBranch                 = "master"
	pxCentralPostInstallHookJobName           = "pxcentral-post-install-hook"
	quickMaintenancePod                       = "quick-maintenance-repo"
	fullMaintenancePod                        = "full-maintenance-repo"
	jobDeleteTimeout                          = 5 * time.Minute
	jobDeleteRetryTime                        = 10 * time.Second
	podStatusTimeOut                          = 20 * time.Minute
	podStatusRetryTime                        = 30 * time.Second
	licenseCountUpdateTimeout                 = 15 * time.Minute
	licenseCountUpdateRetryTime               = 1 * time.Minute
	podReadyTimeout                           = 30 * time.Minute
	storkPodReadyTimeout                      = 15 * time.Minute
	podReadyRetryTime                         = 30 * time.Second
	namespaceDeleteTimeout                    = 10 * time.Minute
	clusterCreationTimeout                    = 5 * time.Minute
	clusterCreationRetryTime                  = 10 * time.Second
)

var (
	// User should keep updating preRuleApp, postRuleApp, appsWithCRDsAndWebhooks
	preRuleApp                  = []string{"cassandra", "postgres"}
	postRuleApp                 = []string{"cassandra"}
	appsWithCRDsAndWebhooks     = []string{"elasticsearch-crd-webhook"} // The apps which have CRDs and webhooks
	globalAWSBucketName         string
	globalAzureBucketName       string
	globalGCPBucketName         string
	globalNFSBucketName         string
	globalAWSLockedBucketName   string
	globalAzureLockedBucketName string
	globalGCPLockedBucketName   string
	cloudProviders              = []string{"aws"}
	commonPassword              string
	backupPodLabels             = []map[string]string{
		{"app": "px-backup"}, {"app.kubernetes.io/component": "pxcentral-apiserver"},
		{"app.kubernetes.io/component": "pxcentral-backend"},
		{"app.kubernetes.io/component": "pxcentral-frontend"},
		{"app.kubernetes.io/component": "keycloak"},
		{"app.kubernetes.io/component": "pxcentral-lh-middleware"},
		{"app.kubernetes.io/component": "pxcentral-mysql"}}
)

type userRoleAccess struct {
	user     string
	roles    backup.PxBackupRole
	accesses BackupAccess
	context  context.Context
}

type userAccessContext struct {
	user     string
	accesses BackupAccess
	context  context.Context
}

var backupAccessKeyValue = map[BackupAccess]string{
	1: "ViewOnlyAccess",
	2: "RestoreAccess",
	3: "FullAccess",
}

var storkLabel = map[string]string{
	"name": "stork",
}

type BackupAccess int32

type ReplacePolicy_Type int32

const (
	ReplacePolicy_Invalid ReplacePolicy_Type = 0
	ReplacePolicy_Retain  ReplacePolicy_Type = 1
	ReplacePolicy_Delete  ReplacePolicy_Type = 2
)

const (
	ViewOnlyAccess BackupAccess = 1
	RestoreAccess               = 2
	FullAccess                  = 3
)

type ExecutionMode int32

const (
	Sequential ExecutionMode = iota
	Parallel
)

// Set default provider as aws
func getProviders() []string {
	providersStr := os.Getenv("PROVIDERS")
	if providersStr != "" {
		return strings.Split(providersStr, ",")
	}
	return cloudProviders
}

// getPXNamespace fetches px namespace from env else sends backup kube-system
func getPXNamespace() string {
	namespace := os.Getenv("PX_NAMESPACE")
	if namespace != "" {
		return namespace
	}
	return defaultStorkDeploymentNamespace
}

// CreateBackup creates backup and checks for success
func CreateBackup(backupName string, clusterName string, bLocation string, bLocationUID string,
	namespaces []string, labelSelectors map[string]string, orgID string, uid string, preRuleName string,
	preRuleUid string, postRuleName string, postRuleUid string, ctx context.Context) error {
	_, err := CreateBackupByNamespacesWithoutCheck(backupName, clusterName, bLocation, bLocationUID, namespaces, labelSelectors, orgID, uid, preRuleName, preRuleUid, postRuleName, postRuleUid, ctx)
	if err != nil {
		return err
	}
	err = backupSuccessCheck(backupName, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Backup [%s] created successfully", backupName)
	return nil
}

// GetCsiSnapshotClassName returns the name of CSI Volume Snapshot class. Returns the first class if there are multiple
func GetCsiSnapshotClassName() (string, error) {
	var snapShotClasses *volsnapv1.VolumeSnapshotClassList
	var err error
	if snapShotClasses, err = Inst().S.GetAllSnapshotClasses(); err != nil {
		return "", err
	}
	if len(snapShotClasses.Items) > 0 {
		log.InfoD("Volume snapshot classes found - ")
		for _, snapshotClass := range snapShotClasses.Items {
			log.InfoD(snapshotClass.GetName())
			if strings.Contains(snapshotClass.GetName(), "csi") {
				log.InfoD("CSI volume snapshot class - %s", snapshotClass.GetName())
				return snapshotClass.GetName(), nil
			}
		}
		log.Warnf("no csi volume snapshot classes found")
		return "", nil
	} else {
		log.Warnf("no volume snapshot classes found")
		return "", nil
	}
}

func FilterAppContextsByNamespace(appContexts []*scheduler.Context, namespaces []string) (filteredAppContexts []*scheduler.Context) {
	for _, appContext := range appContexts {
		if Contains(namespaces, appContext.ScheduleOptions.Namespace) {
			filteredAppContexts = append(filteredAppContexts, appContext)
		}
	}
	return
}

// CreateBackupWithValidation creates backup, checks for success, and validates the backup
func CreateBackupWithValidation(ctx context.Context, backupName string, clusterName string, bLocation string, bLocationUID string, scheduledAppContextsToBackup []*scheduler.Context, labelSelectors map[string]string, orgID string, uid string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string) error {
	namespaces := make([]string, 0)
	for _, scheduledAppContext := range scheduledAppContextsToBackup {
		namespace := scheduledAppContext.ScheduleOptions.Namespace
		if !Contains(namespaces, namespace) {
			namespaces = append(namespaces, namespace)
		}
	}
	err := CreateBackup(backupName, clusterName, bLocation, bLocationUID, namespaces, labelSelectors, orgID, uid, preRuleName, preRuleUid, postRuleName, postRuleUid, ctx)
	if err != nil {
		return err
	}
	return ValidateBackup(ctx, backupName, orgID, scheduledAppContextsToBackup, make([]string, 0))
}

func UpdateBackup(backupName string, backupUid string, orgId string, cloudCred string, cloudCredUID string, ctx context.Context) (*api.BackupUpdateResponse, error) {
	backupDriver := Inst().Backup
	bkpUpdateRequest := &api.BackupUpdateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  backupName,
			OrgId: orgId,
			Uid:   backupUid,
		},
		CloudCredential: cloudCred,
		CloudCredentialRef: &api.ObjectRef{
			Name: cloudCred,
			Uid:  cloudCredUID,
		},
	}
	status, err := backupDriver.UpdateBackup(ctx, bkpUpdateRequest)
	return status, err
}

// CreateBackupWithCustomResourceType creates backup with custom resources
func CreateBackupWithCustomResourceType(backupName string, clusterName string, bLocation string, bLocationUID string,
	namespaces []string, labelSelectors map[string]string, orgID string, uid string, preRuleName string,
	preRuleUid string, postRuleName string, postRuleUid string, resourceTypes []string, ctx context.Context) error {

	backupDriver := Inst().Backup
	bkpCreateRequest := &api.BackupCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  backupName,
			OrgId: orgID,
		},
		BackupLocationRef: &api.ObjectRef{
			Name: bLocation,
			Uid:  bLocationUID,
		},
		Cluster:        clusterName,
		Namespaces:     namespaces,
		LabelSelectors: labelSelectors,
		ClusterRef: &api.ObjectRef{
			Name: clusterName,
			Uid:  uid,
		},
		PreExecRuleRef: &api.ObjectRef{
			Name: preRuleName,
			Uid:  preRuleUid,
		},
		PostExecRuleRef: &api.ObjectRef{
			Name: postRuleName,
			Uid:  postRuleUid,
		},
		ResourceTypes: resourceTypes,
	}
	_, err := backupDriver.CreateBackup(ctx, bkpCreateRequest)
	if err != nil {
		return err
	}
	err = backupSuccessCheck(backupName, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Backup [%s] created successfully", backupName)
	return nil
}

// CreateBackupWithCustomResourceTypeWithValidation creates backup with custom resources selected through resourceTypesFilter, checks for success, and validates the backup
func CreateBackupWithCustomResourceTypeWithValidation(ctx context.Context, backupName string, clusterName string, bLocation string, bLocationUID string, scheduledAppContextsToBackup []*scheduler.Context, resourceTypesFilter []string, labelSelectors map[string]string, orgID string, uid string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string) error {
	namespaces := make([]string, 0)
	for _, scheduledAppContext := range scheduledAppContextsToBackup {
		namespace := scheduledAppContext.ScheduleOptions.Namespace
		if !Contains(namespaces, namespace) {
			namespaces = append(namespaces, namespace)
		}
	}
	err := CreateBackupWithCustomResourceType(backupName, clusterName, bLocation, bLocationUID, namespaces, labelSelectors, orgID, uid, preRuleName, preRuleUid, postRuleName, postRuleUid, resourceTypesFilter, ctx)
	if err != nil {
		return err
	}
	return ValidateBackup(ctx, backupName, orgID, scheduledAppContextsToBackup, resourceTypesFilter)
}

// CreateScheduleBackup creates a schedule backup and checks for success of first (immediately triggered) backup
func CreateScheduleBackup(scheduleName string, clusterName string, bLocation string, bLocationUID string,
	namespaces []string, labelSelectors map[string]string, orgID string, preRuleName string,
	preRuleUid string, postRuleName string, postRuleUid string, schPolicyName string, schPolicyUID string, ctx context.Context) error {
	_, err := CreateScheduleBackupWithoutCheck(scheduleName, clusterName, bLocation, bLocationUID, namespaces, labelSelectors, orgID, preRuleName, preRuleUid, postRuleName, postRuleUid, schPolicyName, schPolicyUID, ctx)
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Minute)
	firstScheduleBackupName, err := GetFirstScheduleBackupName(ctx, scheduleName, orgID)
	if err != nil {
		return err
	}
	err = backupSuccessCheck(firstScheduleBackupName, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Schedule backup [%s] created successfully", firstScheduleBackupName)
	return nil
}

// CreateScheduleBackupWithValidation creates a schedule backup, checks for success of first (immediately triggered) backup, validates that backup and returns the name of that first scheduled backup
func CreateScheduleBackupWithValidation(ctx context.Context, scheduleName string, clusterName string, bLocation string, bLocationUID string, scheduledAppContextsToBackup []*scheduler.Context, labelSelectors map[string]string, orgID string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string, schPolicyName string, schPolicyUID string) (string, error) {
	namespaces := make([]string, 0)
	for _, scheduledAppContext := range scheduledAppContextsToBackup {
		namespace := scheduledAppContext.ScheduleOptions.Namespace
		if !Contains(namespaces, namespace) {
			namespaces = append(namespaces, namespace)
		}
	}
	_, err := CreateScheduleBackupWithoutCheck(scheduleName, clusterName, bLocation, bLocationUID, namespaces, labelSelectors, orgID, preRuleName, preRuleUid, postRuleName, postRuleUid, schPolicyName, schPolicyUID, ctx)
	if err != nil {
		return "", err
	}
	time.Sleep(1 * time.Minute)
	firstScheduleBackupName, err := GetFirstScheduleBackupName(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	log.InfoD("first schedule backup for schedule name [%s] is [%s]", scheduleName, firstScheduleBackupName)
	return firstScheduleBackupName, backupSuccessCheckWithValidation(ctx, firstScheduleBackupName, scheduledAppContextsToBackup, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second)
}

// CreateBackupByNamespacesWithoutCheck creates backup of provided namespaces without waiting for success.
func CreateBackupByNamespacesWithoutCheck(backupName string, clusterName string, bLocation string, bLocationUID string,
	namespaces []string, labelSelectors map[string]string, orgID string, uid string, preRuleName string,
	preRuleUid string, postRuleName string, postRuleUid string, ctx context.Context) (*api.BackupInspectResponse, error) {

	backupDriver := Inst().Backup
	bkpCreateRequest := &api.BackupCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  backupName,
			OrgId: orgID,
		},
		BackupLocationRef: &api.ObjectRef{
			Name: bLocation,
			Uid:  bLocationUID,
		},
		Cluster:        clusterName,
		Namespaces:     namespaces,
		LabelSelectors: labelSelectors,
		ClusterRef: &api.ObjectRef{
			Name: clusterName,
			Uid:  uid,
		},
		PreExecRuleRef: &api.ObjectRef{
			Name: preRuleName,
			Uid:  preRuleUid,
		},
		PostExecRuleRef: &api.ObjectRef{
			Name: postRuleName,
			Uid:  postRuleUid,
		},
	}

	if strings.ToLower(os.Getenv("BACKUP_TYPE")) == "generic" {
		log.Infof("Detected generic backup type")
		bkpCreateRequest.BackupType = api.BackupCreateRequest_Generic
		var csiSnapshotClassName string
		var err error
		if csiSnapshotClassName, err = GetCsiSnapshotClassName(); err != nil {
			return nil, err
		}
		bkpCreateRequest.CsiSnapshotClassName = csiSnapshotClassName
	}

	_, err := backupDriver.CreateBackup(ctx, bkpCreateRequest)
	if err != nil {
		return nil, err
	}
	backupUid, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return nil, err
	}
	backupInspectRequest := &api.BackupInspectRequest{
		Name:  backupName,
		Uid:   backupUid,
		OrgId: orgID,
	}
	resp, err := backupDriver.InspectBackup(ctx, backupInspectRequest)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// CreateBackupWithoutCheck creates backup without waiting for success
func CreateBackupWithoutCheck(ctx context.Context, backupName string, clusterName string, bLocation string, bLocationUID string, scheduledAppContextsToBackup []*scheduler.Context, labelSelectors map[string]string, orgID string, uid string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string) (*api.BackupInspectResponse, error) {
	namespaces := make([]string, 0)
	for _, scheduledAppContext := range scheduledAppContextsToBackup {
		namespace := scheduledAppContext.ScheduleOptions.Namespace
		if !Contains(namespaces, namespace) {
			namespaces = append(namespaces, namespace)
		}
	}

	return CreateBackupByNamespacesWithoutCheck(backupName, clusterName, bLocation, bLocationUID, namespaces, labelSelectors, orgID, uid, preRuleName, preRuleUid, postRuleName, postRuleUid, ctx)
}

// CreateScheduleBackupWithoutCheck creates a schedule backup without waiting for success
func CreateScheduleBackupWithoutCheck(scheduleName string, clusterName string, bLocation string, bLocationUID string,
	namespaces []string, labelSelectors map[string]string, orgID string, preRuleName string,
	preRuleUid string, postRuleName string, postRuleUid string, schPolicyName string, schPolicyUID string, ctx context.Context) (*api.BackupScheduleInspectResponse, error) {
	backupDriver := Inst().Backup
	bkpSchCreateRequest := &api.BackupScheduleCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  scheduleName,
			OrgId: orgID,
		},
		SchedulePolicyRef: &api.ObjectRef{
			Name: schPolicyName,
			Uid:  schPolicyUID,
		},
		BackupLocationRef: &api.ObjectRef{
			Name: bLocation,
			Uid:  bLocationUID,
		},
		SchedulePolicy: schPolicyName,
		Cluster:        clusterName,
		Namespaces:     namespaces,
		LabelSelectors: labelSelectors,
		PreExecRuleRef: &api.ObjectRef{
			Name: preRuleName,
			Uid:  preRuleUid,
		},
		PostExecRuleRef: &api.ObjectRef{
			Name: postRuleName,
			Uid:  postRuleUid,
		},
	}
	_, err := backupDriver.CreateBackupSchedule(ctx, bkpSchCreateRequest)
	if err != nil {
		return nil, err
	}
	backupScheduleInspectRequest := &api.BackupScheduleInspectRequest{
		OrgId: orgID,
		Name:  scheduleName,
		Uid:   "",
	}
	resp, err := backupDriver.InspectBackupSchedule(ctx, backupScheduleInspectRequest)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// ShareBackup provides access to the mentioned groups or/add users
func ShareBackup(backupName string, groupNames []string, userNames []string, accessLevel BackupAccess, ctx context.Context) error {
	var bkpUid string
	backupDriver := Inst().Backup
	groupIDs := make([]string, 0)
	userIDs := make([]string, 0)

	bkpUid, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return err
	}
	log.Infof("Backup UID for %s - %s", backupName, bkpUid)

	for _, groupName := range groupNames {
		groupID, err := backup.FetchIDOfGroup(groupName)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, groupID)
	}

	for _, userName := range userNames {
		userID, err := backup.FetchIDOfUser(userName)
		if err != nil {
			return err
		}
		userIDs = append(userIDs, userID)
	}

	groupBackupShareAccessConfigs := make([]*api.BackupShare_AccessConfig, 0)

	for _, groupName := range groupNames {
		groupBackupShareAccessConfig := &api.BackupShare_AccessConfig{
			Id:     groupName,
			Access: api.BackupShare_AccessType(accessLevel),
		}
		groupBackupShareAccessConfigs = append(groupBackupShareAccessConfigs, groupBackupShareAccessConfig)
	}

	userBackupShareAccessConfigs := make([]*api.BackupShare_AccessConfig, 0)

	for _, userID := range userIDs {
		userBackupShareAccessConfig := &api.BackupShare_AccessConfig{
			Id:     userID,
			Access: api.BackupShare_AccessType(accessLevel),
		}
		userBackupShareAccessConfigs = append(userBackupShareAccessConfigs, userBackupShareAccessConfig)
	}

	shareBackupRequest := &api.BackupShareUpdateRequest{
		OrgId: orgID,
		Name:  backupName,
		Backupshare: &api.BackupShare{
			Groups:        groupBackupShareAccessConfigs,
			Collaborators: userBackupShareAccessConfigs,
		},
		Uid: bkpUid,
	}

	_, err = backupDriver.UpdateBackupShare(ctx, shareBackupRequest)
	return err

}

// ClusterUpdateBackupShare shares all backup with the users and/or groups provided for a given cluster
// addUsersOrGroups - provide true if the mentioned users/groups needs to be added
// addUsersOrGroups - provide false if the mentioned users/groups needs to be deleted or removed
func ClusterUpdateBackupShare(clusterName string, groupNames []string, userNames []string, accessLevel BackupAccess, addUsersOrGroups bool, ctx context.Context) error {
	backupDriver := Inst().Backup
	groupIDs := make([]string, 0)
	userIDs := make([]string, 0)
	clusterUID, err := backupDriver.GetClusterUID(ctx, orgID, clusterName)
	if err != nil {
		return err
	}

	for _, groupName := range groupNames {
		groupID, err := backup.FetchIDOfGroup(groupName)
		if err != nil {
			return err
		}
		groupIDs = append(groupIDs, groupID)
	}

	for _, userName := range userNames {
		userID, err := backup.FetchIDOfUser(userName)
		if err != nil {
			return err
		}
		userIDs = append(userIDs, userID)
	}

	groupBackupShareAccessConfigs := make([]*api.BackupShare_AccessConfig, 0)

	for _, groupName := range groupNames {
		groupBackupShareAccessConfig := &api.BackupShare_AccessConfig{
			Id:     groupName,
			Access: api.BackupShare_AccessType(accessLevel),
		}
		groupBackupShareAccessConfigs = append(groupBackupShareAccessConfigs, groupBackupShareAccessConfig)
	}

	userBackupShareAccessConfigs := make([]*api.BackupShare_AccessConfig, 0)

	for _, userID := range userIDs {
		userBackupShareAccessConfig := &api.BackupShare_AccessConfig{
			Id:     userID,
			Access: api.BackupShare_AccessType(accessLevel),
		}
		userBackupShareAccessConfigs = append(userBackupShareAccessConfigs, userBackupShareAccessConfig)
	}

	backupShare := &api.BackupShare{
		Groups:        groupBackupShareAccessConfigs,
		Collaborators: userBackupShareAccessConfigs,
	}

	var clusterBackupShareUpdateRequest *api.ClusterBackupShareUpdateRequest

	if addUsersOrGroups {
		clusterBackupShareUpdateRequest = &api.ClusterBackupShareUpdateRequest{
			OrgId:          orgID,
			Name:           clusterName,
			AddBackupShare: backupShare,
			DelBackupShare: nil,
			Uid:            clusterUID,
		}
	} else {
		clusterBackupShareUpdateRequest = &api.ClusterBackupShareUpdateRequest{
			OrgId:          orgID,
			Name:           clusterName,
			AddBackupShare: nil,
			DelBackupShare: backupShare,
			Uid:            clusterUID,
		}
	}

	_, err = backupDriver.ClusterUpdateBackupShare(ctx, clusterBackupShareUpdateRequest)
	if err != nil {
		return err
	}

	clusterBackupShareStatusCheck := func() (interface{}, bool, error) {
		clusterReq := &api.ClusterInspectRequest{OrgId: orgID, Name: clusterName, IncludeSecrets: true}
		clusterResp, err := backupDriver.InspectCluster(ctx, clusterReq)
		if err != nil {
			return "", true, err
		}
		if clusterResp.GetCluster().BackupShareStatusInfo.GetStatus() != api.ClusterInfo_BackupShareStatusInfo_Success {
			return "", true, fmt.Errorf("cluster backup share status for cluster %s is still %s", clusterName,
				clusterResp.GetCluster().BackupShareStatusInfo.GetStatus())
		}
		log.Infof("Cluster %s has status - [%d]", clusterName, clusterResp.GetCluster().BackupShareStatusInfo.GetStatus())
		return "", false, nil
	}
	_, err = task.DoRetryWithTimeout(clusterBackupShareStatusCheck, 1*time.Minute, 10*time.Second)
	if err != nil {
		return err
	}
	log.Infof("Cluster backup share check complete")
	return nil
}

func GetAllBackupsForUser(username, password string) ([]string, error) {
	backupNames := make([]string, 0)
	backupDriver := Inst().Backup
	ctx, err := backup.GetNonAdminCtx(username, password)
	if err != nil {
		return nil, err
	}

	backupEnumerateReq := &api.BackupEnumerateRequest{
		OrgId: orgID,
	}
	currentBackups, err := backupDriver.EnumerateBackup(ctx, backupEnumerateReq)
	if err != nil {
		return nil, err
	}
	for _, backup := range currentBackups.GetBackups() {
		backupNames = append(backupNames, backup.GetName())
	}
	return backupNames, nil
}

// CreateRestore creates restore
func CreateRestore(restoreName string, backupName string, namespaceMapping map[string]string, clusterName string,
	orgID string, ctx context.Context, storageClassMapping map[string]string) error {

	var bkp *api.BackupObject
	var bkpUid string
	backupDriver := Inst().Backup
	log.Infof("Getting the UID of the backup %s needed to be restored", backupName)
	bkpEnumerateReq := &api.BackupEnumerateRequest{
		OrgId: orgID}
	curBackups, err := backupDriver.EnumerateBackup(ctx, bkpEnumerateReq)
	if err != nil {
		return err
	}
	for _, bkp = range curBackups.GetBackups() {
		if bkp.Name == backupName {
			bkpUid = bkp.Uid
			break
		}
	}
	createRestoreReq := &api.RestoreCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  restoreName,
			OrgId: orgID,
		},
		Backup:              backupName,
		Cluster:             clusterName,
		NamespaceMapping:    namespaceMapping,
		StorageClassMapping: storageClassMapping,
		BackupRef: &api.ObjectRef{
			Name: backupName,
			Uid:  bkpUid,
		},
	}
	_, err = backupDriver.CreateRestore(ctx, createRestoreReq)
	if err != nil {
		return err
	}
	err = restoreSuccessCheck(restoreName, orgID, maxWaitPeriodForRestoreCompletionInMinute*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Restore [%s] created successfully", restoreName)
	return nil
}

// CreateRestoreWithReplacePolicy Creates in-place restore and waits for it to complete
func CreateRestoreWithReplacePolicy(restoreName string, backupName string, namespaceMapping map[string]string, clusterName string,
	orgID string, ctx context.Context, storageClassMapping map[string]string, replacePolicy ReplacePolicy_Type) error {

	var bkp *api.BackupObject
	var bkpUid string
	backupDriver := Inst().Backup
	log.Infof("Getting the UID of the backup %s needed to be restored", backupName)
	bkpEnumerateReq := &api.BackupEnumerateRequest{
		OrgId: orgID}
	curBackups, err := backupDriver.EnumerateBackup(ctx, bkpEnumerateReq)
	if err != nil {
		return err
	}
	for _, bkp = range curBackups.GetBackups() {
		if bkp.Name == backupName {
			bkpUid = bkp.Uid
			break
		}
	}
	createRestoreReq := &api.RestoreCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  restoreName,
			OrgId: orgID,
		},
		Backup:              backupName,
		Cluster:             clusterName,
		NamespaceMapping:    namespaceMapping,
		StorageClassMapping: storageClassMapping,
		BackupRef: &api.ObjectRef{
			Name: backupName,
			Uid:  bkpUid,
		},
		ReplacePolicy: api.ReplacePolicy_Type(replacePolicy),
	}
	_, err = backupDriver.CreateRestore(ctx, createRestoreReq)
	if err != nil {
		return err
	}
	err = restoreSuccessWithReplacePolicy(restoreName, orgID, maxWaitPeriodForRestoreCompletionInMinute*time.Minute, 30*time.Second, ctx, replacePolicy)
	if err != nil {
		return err
	}
	log.Infof("Restore [%s] created successfully", restoreName)
	return nil
}

// CreateRestoreWithUID creates restore with UID
func CreateRestoreWithUID(restoreName string, backupName string, namespaceMapping map[string]string, clusterName string,
	orgID string, ctx context.Context, storageClassMapping map[string]string, backupUID string) error {

	backupDriver := Inst().Backup
	log.Infof("Getting the UID of the backup needed to be restored")

	createRestoreReq := &api.RestoreCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  restoreName,
			OrgId: orgID,
		},
		Backup:              backupName,
		Cluster:             clusterName,
		NamespaceMapping:    namespaceMapping,
		StorageClassMapping: storageClassMapping,
		BackupRef: &api.ObjectRef{
			Name: backupName,
			Uid:  backupUID,
		},
	}
	_, err := backupDriver.CreateRestore(ctx, createRestoreReq)
	if err != nil {
		return err
	}
	err = restoreSuccessCheck(restoreName, orgID, maxWaitPeriodForRestoreCompletionInMinute*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Restore [%s] created successfully", restoreName)
	return nil
}

// CreateRestoreWithoutCheck creates restore without waiting for completion
func CreateRestoreWithoutCheck(restoreName string, backupName string,
	namespaceMapping map[string]string, clusterName string, orgID string, ctx context.Context) (*api.RestoreInspectResponse, error) {

	var bkp *api.BackupObject
	var bkpUid string
	backupDriver := Inst().Backup
	log.Infof("Getting the UID of the backup needed to be restored")
	bkpEnumerateReq := &api.BackupEnumerateRequest{
		OrgId: orgID}
	curBackups, _ := backupDriver.EnumerateBackup(ctx, bkpEnumerateReq)
	log.Debugf("Enumerate backup response -\n%v", curBackups)
	for _, bkp = range curBackups.GetBackups() {
		if bkp.Name == backupName {
			bkpUid = bkp.Uid
			break
		}
	}
	createRestoreReq := &api.RestoreCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  restoreName,
			OrgId: orgID,
		},
		Backup:           backupName,
		Cluster:          clusterName,
		NamespaceMapping: namespaceMapping,
		BackupRef: &api.ObjectRef{
			Name: backupName,
			Uid:  bkpUid,
		},
	}
	_, err := backupDriver.CreateRestore(ctx, createRestoreReq)
	if err != nil {
		return nil, err
	}
	backupScheduleInspectRequest := &api.RestoreInspectRequest{
		OrgId: orgID,
		Name:  restoreName,
	}
	resp, err := backupDriver.InspectRestore(ctx, backupScheduleInspectRequest)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateRestoreWithValidation creates restore, waits and checks for success and validates the backup
func CreateRestoreWithValidation(ctx context.Context, restoreName, backupName string, namespaceMapping, storageClassMapping map[string]string, clusterName string, orgID string, scheduledAppContexts []*scheduler.Context) (err error) {
	err = CreateRestore(restoreName, backupName, namespaceMapping, clusterName, orgID, ctx, storageClassMapping)
	if err != nil {
		return
	}
	originalClusterConfigPath := CurrentClusterConfigPath
	if clusterConfigPath, ok := ClusterConfigPathMap[clusterName]; !ok {
		err = fmt.Errorf("switching cluster context: couldn't find clusterConfigPath for cluster [%s]", clusterName)
		return
	} else {
		log.InfoD("Switching cluster context to cluster [%s]", clusterName)
		err = SetClusterContext(clusterConfigPath)
		if err != nil {
			return
		}
	}
	defer func() {
		log.InfoD("Switching cluster context back to cluster path [%s]", originalClusterConfigPath)
		err = SetClusterContext(originalClusterConfigPath)
	}()
	expectedRestoredAppContexts := make([]*scheduler.Context, 0)
	for _, scheduledAppContext := range scheduledAppContexts {
		expectedRestoredAppContext, err := CloneAppContextAndTransformWithMappings(scheduledAppContext, namespaceMapping, storageClassMapping, true)
		if err != nil {
			log.Errorf("TransformAppContextWithMappings: %v", err)
			continue
		}
		expectedRestoredAppContexts = append(expectedRestoredAppContexts, expectedRestoredAppContext)
	}
	err = ValidateRestore(ctx, restoreName, orgID, expectedRestoredAppContexts, make([]string, 0))
	return
}

func getSizeOfMountPoint(podName string, namespace string, kubeConfigFile string) (int, error) {
	var number int
	ret, err := kubectlExec([]string{fmt.Sprintf("--kubeconfig=%v", kubeConfigFile), "exec", "-it", podName, "-n", namespace, "--", "/bin/df"})
	if err != nil {
		return 0, err
	}
	for _, line := range strings.SplitAfter(ret, "\n") {
		if strings.Contains(line, "pxd") {
			ret = strings.Fields(line)[3]
		}
	}
	number, err = strconv.Atoi(ret)
	if err != nil {
		return 0, err
	}
	return number, nil
}

func kubectlExec(arguments []string) (string, error) {
	if len(arguments) == 0 {
		return "", fmt.Errorf("no arguments supplied for kubectl command")
	}
	cmd := exec.Command("kubectl", arguments...)
	output, err := cmd.Output()
	log.InfoD("Command '%s'", cmd.String())
	log.Infof("Command output for '%s': %s", cmd.String(), string(output))
	if err != nil {
		return "", fmt.Errorf("error on executing kubectl command, Err: %+v", err)
	}
	return string(output), err
}

func createUsers(numberOfUsers int) []string {
	users := make([]string, 0)
	log.InfoD("Creating %d users", numberOfUsers)
	var wg sync.WaitGroup
	for i := 1; i <= numberOfUsers; i++ {
		userName := fmt.Sprintf("testuser%v-%v", i, time.Now().Unix())
		firstName := fmt.Sprintf("FirstName%v", i)
		lastName := fmt.Sprintf("LastName%v", i)
		email := fmt.Sprintf("%v@cnbu.com", userName)
		wg.Add(1)
		go func(userName, firstName, lastName, email string) {
			defer GinkgoRecover()
			defer wg.Done()
			err := backup.AddUser(userName, firstName, lastName, email, commonPassword)
			Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Creating user - %s", userName))
			users = append(users, userName)
		}(userName, firstName, lastName, email)
	}
	wg.Wait()
	return users
}

// CleanupCloudSettingsAndClusters removes the backup location(s), cloud accounts and source/destination clusters for the given context
func CleanupCloudSettingsAndClusters(backupLocationMap map[string]string, credName string, cloudCredUID string, ctx context.Context) {
	log.InfoD("Cleaning backup locations in map [%v], cloud credential [%s], source [%s] and destination [%s] cluster", backupLocationMap, credName, SourceClusterName, destinationClusterName)
	if len(backupLocationMap) != 0 {
		for backupLocationUID, bkpLocationName := range backupLocationMap {
			// Delete the backup location object
			err := DeleteBackupLocation(bkpLocationName, backupLocationUID, orgID, true)
			Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Verifying deletion of backup location [%s]", bkpLocationName))
			backupLocationDeleteStatusCheck := func() (interface{}, bool, error) {
				status, err := IsBackupLocationPresent(bkpLocationName, ctx, orgID)
				if err != nil {
					return "", true, fmt.Errorf("backup location %s still present with error %v", bkpLocationName, err)
				}
				if status {
					backupLocationInspectRequest := api.BackupLocationInspectRequest{
						Name:  bkpLocationName,
						Uid:   backupLocationUID,
						OrgId: orgID,
					}
					backupLocationObject, err := Inst().Backup.InspectBackupLocation(ctx, &backupLocationInspectRequest)
					if err != nil {
						return "", true, fmt.Errorf("inspect backup location - backup location %s still present with error %v", bkpLocationName, err)
					}
					backupLocationStatus := backupLocationObject.BackupLocation.BackupLocationInfo.GetStatus()
					return "", true, fmt.Errorf("backup location %s is not deleted yet. Status - [%s]", bkpLocationName, backupLocationStatus)
				}
				return "", false, nil
			}
			_, err = task.DoRetryWithTimeout(backupLocationDeleteStatusCheck, backupLocationDeleteTimeout, backupLocationDeleteRetryTime)
			Inst().Dash.VerifySafely(err, nil, fmt.Sprintf("Verifying backup location deletion status %s", bkpLocationName))
		}
		status, err := IsCloudCredPresent(credName, ctx, orgID)
		Inst().Dash.VerifySafely(err, nil, fmt.Sprintf("Verifying if cloud cred [%s] is present", credName))
		if status {
			err = DeleteCloudCredential(credName, orgID, cloudCredUID)
			Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Verifying deletion of cloud cred [%s]", credName))
			cloudCredDeleteStatus := func() (interface{}, bool, error) {
				status, err = IsCloudCredPresent(credName, ctx, orgID)
				if err != nil {
					return "", true, fmt.Errorf("cloud cred %s still present with error %v", credName, err)
				}
				if status {
					return "", true, fmt.Errorf("cloud cred %s is not deleted yet", credName)
				}
				return "", false, nil
			}
			_, err = task.DoRetryWithTimeout(cloudCredDeleteStatus, cloudAccountDeleteTimeout, cloudAccountDeleteRetryTime)
			Inst().Dash.VerifySafely(err, nil, fmt.Sprintf("Deleting cloud cred %s", credName))
		}
	}
	err := DeleteCluster(SourceClusterName, orgID, ctx)
	Inst().Dash.VerifySafely(err, nil, fmt.Sprintf("Deleting cluster %s", SourceClusterName))
	err = DeleteCluster(destinationClusterName, orgID, ctx)
	Inst().Dash.VerifySafely(err, nil, fmt.Sprintf("Deleting cluster %s", destinationClusterName))
}

// AddRoleAndAccessToUsers assigns role and access level to the users
// AddRoleAndAccessToUsers return map whose key is userRoleAccess and value is backup for each user
func AddRoleAndAccessToUsers(users []string, backupNames []string) (map[userRoleAccess]string, error) {
	var access BackupAccess
	var role backup.PxBackupRole
	roleAccessUserBackupContext := make(map[userRoleAccess]string)
	ctx, err := backup.GetAdminCtxFromSecret()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(users); i++ {
		userIndex := i % 9
		switch userIndex {
		case 0:
			access = ViewOnlyAccess
			role = backup.ApplicationOwner
		case 1:
			access = RestoreAccess
			role = backup.ApplicationOwner
		case 2:
			access = FullAccess
			role = backup.ApplicationOwner
		case 3:
			access = ViewOnlyAccess
			role = backup.ApplicationUser
		case 4:
			access = RestoreAccess
			role = backup.ApplicationUser
		case 5:
			access = FullAccess
			role = backup.ApplicationUser
		case 6:
			access = ViewOnlyAccess
			role = backup.InfrastructureOwner
		case 7:
			access = RestoreAccess
			role = backup.InfrastructureOwner
		case 8:
			access = FullAccess
			role = backup.InfrastructureOwner
		default:
			access = ViewOnlyAccess
			role = backup.ApplicationOwner
		}
		ctxNonAdmin, err := backup.GetNonAdminCtx(users[i], commonPassword)
		if err != nil {
			return nil, err
		}
		userRoleAccessContext := userRoleAccess{users[i], role, access, ctxNonAdmin}
		roleAccessUserBackupContext[userRoleAccessContext] = backupNames[i]
		err = backup.AddRoleToUser(users[i], role, "Adding role to user")
		if err != nil {
			err = fmt.Errorf("failed to add role %s to user %s with err %v", role, users[i], err)
			return nil, err
		}
		err = ShareBackup(backupNames[i], nil, []string{users[i]}, access, ctx)
		if err != nil {
			return nil, err
		}
		log.Infof(" Backup %s shared with user %s", backupNames[i], users[i])
	}
	return roleAccessUserBackupContext, nil
}
func ValidateSharedBackupWithUsers(user string, access BackupAccess, backupName string, restoreName string) {
	ctx, err := backup.GetAdminCtxFromSecret()
	Inst().Dash.VerifyFatal(err, nil, "Fetching px-central-admin ctx")
	userCtx, err := backup.GetNonAdminCtx(user, commonPassword)
	Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Fetching %s user ctx", user))
	log.InfoD("Registering Source and Destination clusters from user context")
	err = CreateSourceAndDestClusters(orgID, "", "", userCtx)
	Inst().Dash.VerifyFatal(err, nil, "Creating source and destination cluster")
	log.InfoD("Validating if user [%s] with access [%v] can restore and delete backup %s or not", user, backupAccessKeyValue[access], backupName)
	backupDriver := Inst().Backup
	switch access {
	case ViewOnlyAccess:
		// Try restore with user having ViewOnlyAccess and it should fail
		err := CreateRestore(restoreName, backupName, make(map[string]string), destinationClusterName, orgID, userCtx, make(map[string]string))
		log.Infof("The expected error returned is %v", err)
		Inst().Dash.VerifyFatal(strings.Contains(err.Error(), "failed to retrieve backup location"), true, "Verifying backup restore is not possible")
		// Try to delete the backup with user having ViewOnlyAccess, and it should not pass
		backupUID, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
		Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Getting backup UID for- %s", backupName))
		// Delete backup to confirm that the user has ViewOnlyAccess and cannot delete backup
		_, err = DeleteBackup(backupName, backupUID, orgID, userCtx)
		log.Infof("The expected error returned is %v", err)
		Inst().Dash.VerifyFatal(strings.Contains(err.Error(), "doesn't have permission to delete backup"), true, "Verifying backup deletion is not possible")

	case RestoreAccess:
		// Try restore with user having RestoreAccess and it should pass
		err := CreateRestore(restoreName, backupName, make(map[string]string), destinationClusterName, orgID, userCtx, make(map[string]string))
		Inst().Dash.VerifyFatal(err, nil, "Verifying that restore is possible")
		// Try to delete the backup with user having RestoreAccess, and it should not pass
		backupUID, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
		Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Getting backup UID for- %s", backupName))
		// Delete backup to confirm that the user has Restore Access and delete backup should fail
		_, err = DeleteBackup(backupName, backupUID, orgID, userCtx)
		log.Infof("The expected error returned is %v", err)
		Inst().Dash.VerifyFatal(strings.Contains(err.Error(), "doesn't have permission to delete backup"), true, "Verifying backup deletion is not possible")

	case FullAccess:
		// Try restore with user having FullAccess, and it should pass
		err := CreateRestore(restoreName, backupName, make(map[string]string), destinationClusterName, orgID, userCtx, make(map[string]string))
		Inst().Dash.VerifyFatal(err, nil, "Verifying that restore is possible")
		// Try to delete the backup with user having FullAccess, and it should pass
		backupUID, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
		Inst().Dash.VerifyFatal(err, nil, fmt.Sprintf("Getting backup UID for- %s", backupName))
		// Delete backup to confirm that the user has Full Access
		_, err = DeleteBackup(backupName, backupUID, orgID, userCtx)
		Inst().Dash.VerifyFatal(err, nil, "Verifying that delete backup is possible")
	}
}

func getEnv(environmentVariable string, defaultValue string) string {
	value, present := os.LookupEnv(environmentVariable)
	if !present {
		value = defaultValue
	}
	return value
}

// ShareBackupWithUsersAndAccessAssignment shares backup with multiple users with different access levels
// It returns a map with key as userAccessContext and value as backup shared
func ShareBackupWithUsersAndAccessAssignment(backupNames []string, users []string, ctx context.Context) (map[userAccessContext]string, error) {
	log.InfoD("Sharing backups with users with different access level")
	accessUserBackupContext := make(map[userAccessContext]string)
	var err error
	var ctxNonAdmin context.Context
	var access BackupAccess
	for i, user := range users {
		userIndex := i % 3
		switch userIndex {
		case 0:
			access = ViewOnlyAccess
			err = ShareBackup(backupNames[i], nil, []string{user}, access, ctx)
		case 1:
			access = RestoreAccess
			err = ShareBackup(backupNames[i], nil, []string{user}, access, ctx)
		case 2:
			access = FullAccess
			err = ShareBackup(backupNames[i], nil, []string{user}, access, ctx)
		default:
			access = ViewOnlyAccess
			err = ShareBackup(backupNames[i], nil, []string{user}, access, ctx)
		}
		if err != nil {
			return accessUserBackupContext, fmt.Errorf("unable to share backup %s with user %s Error: %v", backupNames[i], user, err)
		}
		ctxNonAdmin, err = backup.GetNonAdminCtx(users[i], commonPassword)
		if err != nil {
			return accessUserBackupContext, fmt.Errorf("unable to get user context: %v", err)
		}
		accessContextUser := userAccessContext{users[i], access, ctxNonAdmin}
		accessUserBackupContext[accessContextUser] = backupNames[i]
	}
	return accessUserBackupContext, nil
}

// GetAllBackupsAdmin returns all the backups that px-central-admin has access to
func GetAllBackupsAdmin() ([]string, error) {
	var bkp *api.BackupObject
	backupNames := make([]string, 0)
	backupDriver := Inst().Backup
	ctx, err := backup.GetAdminCtxFromSecret()
	if err != nil {
		return nil, err
	}
	bkpEnumerateReq := &api.BackupEnumerateRequest{
		OrgId: orgID}
	curBackups, err := backupDriver.EnumerateBackup(ctx, bkpEnumerateReq)
	if err != nil {
		return nil, err
	}
	for _, bkp = range curBackups.GetBackups() {
		backupNames = append(backupNames, bkp.GetName())
	}
	return backupNames, nil
}

// GetAllRestoresAdmin returns all the backups that px-central-admin has access to
func GetAllRestoresAdmin() ([]string, error) {
	restoreNames := make([]string, 0)
	backupDriver := Inst().Backup
	ctx, err := backup.GetAdminCtxFromSecret()
	if err != nil {
		return restoreNames, err
	}

	restoreEnumerateRequest := &api.RestoreEnumerateRequest{
		OrgId: orgID,
	}
	restoreResponse, err := backupDriver.EnumerateRestore(ctx, restoreEnumerateRequest)
	if err != nil {
		return restoreNames, err
	}
	for _, restore := range restoreResponse.GetRestores() {
		restoreNames = append(restoreNames, restore.Name)
	}
	return restoreNames, nil
}

func generateEncryptionKey() string {
	var lower = []byte("abcdefghijklmnopqrstuvwxyz")
	var upper = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	var number = []byte("0123456789")
	var special = []byte("~=+%^*/()[]{}/!@#$?|")
	allChar := append(lower, upper...)
	allChar = append(allChar, number...)
	allChar = append(allChar, special...)

	b := make([]byte, 12)
	// select 1 upper, 1 lower, 1 number and 1 special
	b[0] = lower[rand.Intn(len(lower))]
	b[1] = upper[rand.Intn(len(upper))]
	b[2] = number[rand.Intn(len(number))]
	b[3] = special[rand.Intn(len(special))]
	for i := 4; i < 12; i++ {
		// randomly select 1 character from given charset
		b[i] = allChar[rand.Intn(len(allChar))]
	}

	//shuffle character
	rand.Shuffle(len(b), func(i, j int) {
		b[i], b[j] = b[j], b[i]
	})

	return string(b)
}

func GetScheduleUID(scheduleName string, orgID string, ctx context.Context) (string, error) {
	backupDriver := Inst().Backup
	backupScheduleInspectRequest := &api.BackupScheduleInspectRequest{
		Name:  scheduleName,
		Uid:   "",
		OrgId: orgID,
	}
	resp, err := backupDriver.InspectBackupSchedule(ctx, backupScheduleInspectRequest)
	if err != nil {
		return "", err
	}
	scheduleUid := resp.GetBackupSchedule().GetUid()
	return scheduleUid, err
}

func removeStringItemFromSlice(itemList []string, item []string) []string {
	sort.Sort(sort.StringSlice(itemList))
	for _, element := range item {
		index := sort.StringSlice(itemList).Search(element)
		itemList = append(itemList[:index], itemList[index+1:]...)
	}
	return itemList
}

func removeIntItemFromSlice(itemList []int, item []int) []int {
	sort.Sort(sort.IntSlice(itemList))
	for _, element := range item {
		index := sort.IntSlice(itemList).Search(element)
		itemList = append(itemList[:index], itemList[index+1:]...)
	}
	return itemList
}

func getAllBackupLocations(ctx context.Context) (map[string]string, error) {
	backupLocationMap := make(map[string]string, 0)
	backupDriver := Inst().Backup
	backupLocationEnumerateRequest := &api.BackupLocationEnumerateRequest{
		OrgId: orgID,
	}
	response, err := backupDriver.EnumerateBackupLocation(ctx, backupLocationEnumerateRequest)
	if err != nil {
		return backupLocationMap, err
	}
	if len(response.BackupLocations) > 0 {
		for _, backupLocation := range response.BackupLocations {
			backupLocationMap[backupLocation.Uid] = backupLocation.Name
		}
		log.Infof("The backup locations and their UID are %v", backupLocationMap)
	} else {
		log.Info("No backup locations found")
	}
	return backupLocationMap, nil
}

func getAllCloudCredentials(ctx context.Context) (map[string]string, error) {
	cloudCredentialMap := make(map[string]string, 0)
	backupDriver := Inst().Backup
	cloudCredentialEnumerateRequest := &api.CloudCredentialEnumerateRequest{
		OrgId: orgID,
	}
	response, err := backupDriver.EnumerateCloudCredential(ctx, cloudCredentialEnumerateRequest)
	if err != nil {
		return cloudCredentialMap, err
	}
	if len(response.CloudCredentials) > 0 {
		for _, cloudCredential := range response.CloudCredentials {
			cloudCredentialMap[cloudCredential.Uid] = cloudCredential.Name
		}
		log.Infof("The cloud credentials and their UID are %v", cloudCredentialMap)
	} else {
		log.Info("No cloud credentials found")
	}
	return cloudCredentialMap, nil
}

func GetAllRestoresNonAdminCtx(ctx context.Context) ([]string, error) {
	restoreNames := make([]string, 0)
	backupDriver := Inst().Backup
	restoreEnumerateRequest := &api.RestoreEnumerateRequest{
		OrgId: orgID,
	}
	restoreResponse, err := backupDriver.EnumerateRestore(ctx, restoreEnumerateRequest)
	if err != nil {
		return restoreNames, err
	}
	for _, restore := range restoreResponse.GetRestores() {
		restoreNames = append(restoreNames, restore.Name)
	}
	return restoreNames, nil
}

// DeletePodWithLabelInNamespace kills pod with the given label in the given namespace
func DeletePodWithLabelInNamespace(namespace string, label map[string]string) error {
	pods, err := core.Instance().GetPods(namespace, label)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err := core.Instance().DeletePod(pod.GetName(), namespace, false)
		if err != nil {
			return err
		}
		err = core.Instance().WaitForPodDeletion(pod.GetUID(), namespace, 5*time.Minute)
		if err != nil {
			return err
		}
	}
	return nil
}

// backupSuccessCheck inspects backup task
func backupSuccessCheck(backupName string, orgID string, retryDuration time.Duration, retryInterval time.Duration, ctx context.Context) error {
	bkpUid, err := Inst().Backup.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return err
	}
	backupInspectRequest := &api.BackupInspectRequest{
		Name:  backupName,
		Uid:   bkpUid,
		OrgId: orgID,
	}
	statusesExpected := [...]api.BackupInfo_StatusInfo_Status{
		api.BackupInfo_StatusInfo_Success,
	}
	statusesUnexpected := [...]api.BackupInfo_StatusInfo_Status{
		api.BackupInfo_StatusInfo_Invalid,
		api.BackupInfo_StatusInfo_Aborted,
		api.BackupInfo_StatusInfo_Failed,
	}
	backupSuccessCheckFunc := func() (interface{}, bool, error) {
		resp, err := Inst().Backup.InspectBackup(ctx, backupInspectRequest)
		if err != nil {
			return "", false, err
		}
		actual := resp.GetBackup().GetStatus().Status
		reason := resp.GetBackup().GetStatus().Reason
		for _, status := range statusesExpected {
			if actual == status {
				return "", false, nil
			}
		}
		for _, status := range statusesUnexpected {
			if actual == status {
				return "", false, fmt.Errorf("backup status for [%s] expected was [%s] but got [%s] because of [%s]", backupName, statusesExpected, actual, reason)
			}
		}
		return "", true, fmt.Errorf("backup status for [%s] expected was [%s] but got [%s] because of [%s]", backupName, statusesExpected, actual, reason)

	}
	_, err = task.DoRetryWithTimeout(backupSuccessCheckFunc, retryDuration, retryInterval)
	if err != nil {
		return err
	}
	return nil
}

// backupSuccessCheckWithValidation checks if backup is Success and then validates the backup
func backupSuccessCheckWithValidation(ctx context.Context, backupName string, scheduledAppContextsToBackup []*scheduler.Context, orgID string, retryDuration time.Duration, retryInterval time.Duration) error {
	err := backupSuccessCheck(backupName, orgID, retryDuration, retryInterval, ctx)
	if err != nil {
		return err
	}
	return ValidateBackup(ctx, backupName, orgID, scheduledAppContextsToBackup, make([]string, 0))
}

// ValidateBackup validates a backup's spec's objects (resources) and volumes. resourceTypesFilter can be used to select specific types to validate (nil means all types). This function must be called after switching to the context on which `scheduledAppContexts` exists. Cluster level resources aren't validated.
func ValidateBackup(ctx context.Context, backupName string, orgID string, scheduledAppContexts []*scheduler.Context, resourceTypesFilter []string) error {
	log.InfoD("Validating backup [%s] in org [%s]", backupName, orgID)

	log.Infof("Obtaining backup info for backup [%s]", backupName)
	backupDriver := Inst().Backup
	backupUid, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return fmt.Errorf("GetBackupUID Err: %v", err)
	}
	backupInspectRequest := &api.BackupInspectRequest{
		Name:  backupName,
		Uid:   backupUid,
		OrgId: orgID,
	}
	backupInspectResponse, err := backupDriver.InspectBackup(ctx, backupInspectRequest)
	if err != nil {
		return fmt.Errorf("InspectBackup Err: %v", err)
	}

	backupStatus := backupInspectResponse.GetBackup().GetStatus().Status
	if backupStatus != api.BackupInfo_StatusInfo_Success &&
		backupStatus != api.BackupInfo_StatusInfo_PartialSuccess {
		return fmt.Errorf("ValidateBackup requires backup [%s] to have a status of Success or PartialSuccess", backupName)
	}

	var errors []error

	theBackup := backupInspectResponse.GetBackup()
	backupName = theBackup.GetName()
	resourceInfos := theBackup.GetResources()
	backedupVolumes := theBackup.GetVolumes()
	backupNamespaces := theBackup.GetNamespaces()

	for _, scheduledAppContext := range scheduledAppContexts {

		scheduledAppContextNamespace := scheduledAppContext.ScheduleOptions.Namespace
		log.InfoD("Validating specs for the namespace (scheduledAppContext) [%s] in backup [%s]", scheduledAppContextNamespace, backupName)

		if !Contains(backupNamespaces, scheduledAppContextNamespace) {
			err := fmt.Errorf("the namespace (scheduledAppContext) [%s] provided to the ValidateBackup, is not present in the backup [%s]", scheduledAppContextNamespace, backupName)
			errors = append(errors, err)
			continue
		}

		// collect the backup resources whose specs should be present in this scheduledAppContext (namespace)
		resourceInfoBackupObjs := make([]*api.ResourceInfo, 0)
		for _, resource := range resourceInfos {
			if resource.GetNamespace() == scheduledAppContextNamespace {
				resourceInfoBackupObjs = append(resourceInfoBackupObjs, resource)
			}
		}

	specloop:
		for _, spec := range scheduledAppContext.App.SpecList {

			name, kind, ns, err := GetSpecNameKindNamepace(spec)
			if err != nil {
				err := fmt.Errorf("error in GetSpecNameKindNamepace: [%s] in namespace (appCtx) [%s], spec: [%+v]", err, scheduledAppContextNamespace, spec)
				errors = append(errors, err)
				continue specloop
			}

			if name == "" || kind == "" {
				err := fmt.Errorf("error: GetSpecNameKindNamepace returned values with Spec Name: [%s], Kind: [%s], Namespace: [%s], in local Context (NS): [%s], where some of the values are empty, so this spec will be ignored", name, kind, ns, scheduledAppContextNamespace)
				errors = append(errors, err)
				continue specloop
			}

			if kind == "StorageClass" || kind == "VolumeSnapshot" {
				// we don't backup "StorageClass"s and "VolumeSnapshot"s
				continue specloop
			}

			if len(resourceTypesFilter) > 0 && !Contains(resourceTypesFilter, kind) {
				log.Infof("kind: [%s] is not in resourceTypes [%v], so spec (name: [%s], kind: [%s], namespace: [%s]) in scheduledAppContext [%s] will not be checked for in backup [%s]", kind, resourceTypesFilter, name, kind, ns, scheduledAppContextNamespace, backupName)
				continue specloop
			}

			// we only validate namespace level resource
			if ns != "" {
				for _, backupObj := range resourceInfoBackupObjs {
					if name == backupObj.GetName() && kind == backupObj.GetKind() {
						continue specloop
					}
				}

				// The following error means that something was NOT backed up,
				// OR it wasn't supposed to be backed up, and we forgot to exclude the check.
				err := fmt.Errorf("the spec (name: [%s], kind: [%s], namespace: [%s]) found in the scheduledAppContext [%s], is not in the backup [%s]", name, kind, ns, scheduledAppContextNamespace, backupName)
				errors = append(errors, err)
				continue specloop
			}
		}

		log.InfoD("Validating backed up volumes for the namespace (scheduledAppContext) [%s] in backup [%s]", scheduledAppContextNamespace, backupName)

		// collect the backup resources whose VOLUMES should be present in this scheduledAppContext (namespace)
		namespacedBackedUpVolumes := make([]*api.BackupInfo_Volume, 0)
		for _, vol := range backedupVolumes {
			if vol.GetNamespace() == scheduledAppContextNamespace {
				if vol.Status.Status != api.BackupInfo_StatusInfo_Success /*Can this also be partialsuccess?*/ {
					err := fmt.Errorf("the status of the backedup volume [%s] was not Success. It was [%s] with reason [%s]", vol.Name, vol.Status.Status, vol.Status.Reason)
					errors = append(errors, err)
				}
				namespacedBackedUpVolumes = append(namespacedBackedUpVolumes, vol)
			}
		}

		// Collect all volumes belonging to a context
		log.Infof("getting the volumes bounded to the PVCs in the namespace (scheduledAppContext) [%s]", scheduledAppContextNamespace)
		volumeMap := make(map[string]*volume.Volume)
		scheduledVolumes, err := Inst().S.GetVolumes(scheduledAppContext)
		if err != nil {
			err := fmt.Errorf("error in Inst().S.GetVolumes: [%s] in namespace (appCtx) [%s]", err, scheduledAppContextNamespace)
			errors = append(errors, err)
			continue
		}
		for _, scheduledVol := range scheduledVolumes {
			volumeMap[scheduledVol.ID] = scheduledVol
		}
		log.Infof("volumes bounded to the PVCs in the context [%s] are [%+v]", scheduledAppContextNamespace, scheduledVolumes)

		if len(resourceTypesFilter) == 0 ||
			(len(resourceTypesFilter) > 0 && Contains(resourceTypesFilter, "PersistentVolumeClaim")) {
			// Verify if volumes are present
		volloop:
			for _, spec := range scheduledAppContext.App.SpecList {
				// Obtaining the volume from the PVC
				pvcSpecObj, ok := spec.(*corev1.PersistentVolumeClaim)
				if !ok {
					continue volloop
				}

				sched, ok := Inst().S.(*k8s.K8s)
				if !ok {
					continue volloop
				}

				updatedSpec, err := sched.GetUpdatedSpec(pvcSpecObj)
				if err != nil {
					err := fmt.Errorf("unable to fetch updated version of PVC(name: [%s], namespace: [%s]) present in the context [%s]. Error: %v", pvcSpecObj.GetName(), pvcSpecObj.GetNamespace(), scheduledAppContextNamespace, err)
					errors = append(errors, err)
					continue volloop
				}

				pvcObj, ok := updatedSpec.(*corev1.PersistentVolumeClaim)
				if !ok {
					err := fmt.Errorf("unable to fetch updated version of PVC(name: [%s], namespace: [%s]) present in the context [%s]. Error: %v", pvcSpecObj.GetName(), pvcSpecObj.GetNamespace(), scheduledAppContextNamespace, err)
					errors = append(errors, err)
					continue volloop
				}

				scheduledVol, ok := volumeMap[pvcObj.Spec.VolumeName]
				if !ok {
					err := fmt.Errorf("unable to find the volume corresponding to PVC(name: [%s], namespace: [%s]) in the cluster corresponding to the PVC's context, which is [%s]", pvcSpecObj.GetName(), pvcSpecObj.GetNamespace(), scheduledAppContextNamespace)
					errors = append(errors, err)
					continue volloop
				}

				// Finding the volume in the backup
				for _, backedupVol := range namespacedBackedUpVolumes {
					if backedupVol.GetName() == scheduledVol.ID {

						if backedupVol.Pvc != pvcObj.Name {
							err := fmt.Errorf("the PVC of the volume as per the backup [%s] is [%s], but the one found in the scheduled namesapce is [%s]", backedupVol.GetName(), backedupVol.Pvc, pvcObj.Name)
							errors = append(errors, err)
						}

						var expectedVolumeDriver string
						if strings.ToLower(os.Getenv("BACKUP_TYPE")) == "generic" {
							expectedVolumeDriver = "kdmp"
						} else {
							expectedVolumeDriver = Inst().V.String()
						}

						if backedupVol.DriverName != expectedVolumeDriver {
							err := fmt.Errorf("the Driver Name of the volume as per the backup [%s] is [%s], but the one expected is [%s]", backedupVol.GetName(), backedupVol.DriverName, expectedVolumeDriver)
							errors = append(errors, err)
						}

						if backedupVol.StorageClass != *pvcObj.Spec.StorageClassName {
							err := fmt.Errorf("the Storage Class of the volume as per the backup [%s] is [%s], but the one found in the scheduled namesapce is [%s]", backedupVol.GetName(), backedupVol.StorageClass, *pvcObj.Spec.StorageClassName)
							errors = append(errors, err)
						}

						continue volloop
					}
				}

				// The following error means that something WAS not backed up, OR it wasn't supposed to be backed up, and we forgot to exclude the check.
				err = fmt.Errorf("the volume [%s] corresponding to PVC(name: [%s], namespace: [%s]) was present in the cluster with the namespace containing that PVC, but the volume was not in the backup [%s]", pvcObj.Spec.VolumeName, pvcObj.GetName(), pvcObj.GetNamespace(), backupName)
				errors = append(errors, err)
			}
		} else {
			log.Infof("volumes in scheduledAppContext [%s] will not be checked for in backup [%s] as PersistentVolumeClaims are not backed up", scheduledAppContextNamespace, backupName)
		}

	}

	errStrings := make([]string, 0)
	for _, err := range errors {
		if err != nil {
			errStrings = append(errStrings, err.Error())
		}
	}

	if len(errStrings) > 0 {
		return fmt.Errorf("ValidateBackup Errors: {%s}", strings.Join(errStrings, "}\n{"))
	} else {
		return nil
	}
}

// restoreSuccessCheck inspects restore task to check for status being "success". NOTE: If the status is different, it retries every `retryInterval` for `retryDuration` before returning `err`
func restoreSuccessCheck(restoreName string, orgID string, retryDuration time.Duration, retryInterval time.Duration, ctx context.Context) error {
	restoreInspectRequest := &api.RestoreInspectRequest{
		Name:  restoreName,
		OrgId: orgID,
	}
	statusesExpected := [...]api.RestoreInfo_StatusInfo_Status{
		api.RestoreInfo_StatusInfo_PartialSuccess,
		api.RestoreInfo_StatusInfo_Success,
	}
	statusesUnexpected := [...]api.RestoreInfo_StatusInfo_Status{
		api.RestoreInfo_StatusInfo_Invalid,
		api.RestoreInfo_StatusInfo_Aborted,
		api.RestoreInfo_StatusInfo_Failed,
	}
	restoreSuccessCheckFunc := func() (interface{}, bool, error) {
		resp, err := Inst().Backup.InspectRestore(ctx, restoreInspectRequest)
		if err != nil {
			return "", false, err
		}
		actual := resp.GetRestore().GetStatus().Status
		reason := resp.GetRestore().GetStatus().Reason
		for _, status := range statusesExpected {
			if actual == status {
				return "", false, nil
			}
		}
		for _, status := range statusesUnexpected {
			if actual == status {
				return "", false, fmt.Errorf("restore status for [%s] expected was [%v] but got [%s] because of [%s]", restoreName, statusesExpected, actual, reason)
			}
		}
		return "", true, fmt.Errorf("restore status for [%s] expected was [%v] but got [%s] because of [%s]", restoreName, statusesExpected, actual, reason)
	}
	_, err := task.DoRetryWithTimeout(restoreSuccessCheckFunc, retryDuration, retryInterval)
	if err != nil {
		return err
	}
	return nil
}

// restoreSuccessWithReplacePolicy inspects restore task status as per ReplacePolicy_Type
func restoreSuccessWithReplacePolicy(restoreName string, orgID string, retryDuration time.Duration, retryInterval time.Duration, ctx context.Context, replacePolicy ReplacePolicy_Type) error {
	restoreInspectRequest := &api.RestoreInspectRequest{
		Name:  restoreName,
		OrgId: orgID,
	}
	var statusesExpected api.RestoreInfo_StatusInfo_Status
	if replacePolicy == ReplacePolicy_Delete {
		statusesExpected = api.RestoreInfo_StatusInfo_Success
	} else if replacePolicy == ReplacePolicy_Retain {
		statusesExpected = api.RestoreInfo_StatusInfo_PartialSuccess
	}
	statusesUnexpected := [...]api.RestoreInfo_StatusInfo_Status{
		api.RestoreInfo_StatusInfo_Invalid,
		api.RestoreInfo_StatusInfo_Aborted,
		api.RestoreInfo_StatusInfo_Failed,
	}
	restoreSuccessCheckFunc := func() (interface{}, bool, error) {
		resp, err := Inst().Backup.InspectRestore(ctx, restoreInspectRequest)
		if err != nil {
			return "", false, err
		}
		actual := resp.GetRestore().GetStatus().Status
		reason := resp.GetRestore().GetStatus().Reason
		if actual == statusesExpected {
			return "", false, nil
		}

		for _, status := range statusesUnexpected {
			if actual == status {
				return "", false, fmt.Errorf("restore status for [%s] expected was [%v] but got [%s] because of [%s]", restoreName, statusesExpected, actual, reason)
			}
		}
		return "", true, fmt.Errorf("restore status for [%s] expected was [%v] but got [%s] because of [%s]", restoreName, statusesExpected, actual, reason)
	}
	_, err := task.DoRetryWithTimeout(restoreSuccessCheckFunc, retryDuration, retryInterval)
	return err
}

// ValidateRestore validates a restore's spec's objects (resources) and volumes using expectedRestoredAppContexts (generated by transforming scheduledAppContexts using TransformAppContextWithMappings). This function must be called after switching to the context on which `expectedRestoredAppContexts` exists. Cluster level resources aren't validated.
func ValidateRestore(ctx context.Context, restoreName string, orgID string, expectedRestoredAppContexts []*scheduler.Context, resourceTypesFilter []string) error {
	log.InfoD("Validating restore [%s] in org [%s]", restoreName, orgID)

	log.Infof("Obtaining restore info for restore [%s]", restoreName)
	backupDriver := Inst().Backup
	restoreInspectRequest := &api.RestoreInspectRequest{
		Name:  restoreName,
		OrgId: orgID,
	}
	restoreInspectResponse, err := backupDriver.InspectRestore(ctx, restoreInspectRequest)
	if err != nil {
		return err
	}
	theRestore := restoreInspectResponse.GetRestore()
	restoredResourcesInfo := theRestore.GetResources()
	apparentlyRestoredVolumes := theRestore.GetVolumes()
	namespaceMappings := theRestore.GetNamespaceMapping()

	log.Infof("Checking status of restore [%s]", restoreName)
	restoreStatus := theRestore.GetStatus().Status
	if restoreStatus != api.RestoreInfo_StatusInfo_Success &&
		restoreStatus != api.RestoreInfo_StatusInfo_PartialSuccess {
		restoreStatusReason := theRestore.GetStatus().Reason
		return fmt.Errorf("ValidateRestore requires restore [%s] to have a status of Success or PartialSuccess, but found [%s] with reason [%s]", restoreName, restoreStatus, restoreStatusReason)
	}

	var errors []error

	// check if all the objects in the spec are present in the restore as per what px-backup reports
	for _, expectedRestoredAppContext := range expectedRestoredAppContexts {

		expectedRestoredAppContextNamespace := expectedRestoredAppContext.ScheduleOptions.Namespace
		log.InfoD("Validating specs for the namespace (restoredAppContext) [%s] in restore [%s]", expectedRestoredAppContextNamespace, restoreName)

		NSisPresent := false
		for _, restoredNS := range namespaceMappings {
			if restoredNS == expectedRestoredAppContextNamespace {
				NSisPresent = true
				break
			}
		}
		if !NSisPresent {
			err := fmt.Errorf("the namespace (restoredAppContext) [%s] provided to the ValidateRestore, is apparently not present in the restore [%s], hence cannot validate", expectedRestoredAppContextNamespace, restoreName)
			errors = append(errors, err)
			continue
		}

		// collect the backup resources whose specs should be present in this expectedRestoredAppContext (namespace)
		restoredObjectsInNS := make([]*api.RestoreInfo_RestoredResource, 0)
		for _, resource := range restoredResourcesInfo {
			if resource.GetNamespace() == expectedRestoredAppContextNamespace {
				restoredObjectsInNS = append(restoredObjectsInNS, resource)
			}
		}

	specloop:
		for _, specObj := range expectedRestoredAppContext.App.SpecList {

			name, kind, ns, err := GetSpecNameKindNamepace(specObj)
			if err != nil {
				err := fmt.Errorf("error in GetSpecNameKindNamepace: [%s] in namespace (restoredAppContext) [%s], spec: [%+v]", err, expectedRestoredAppContextNamespace, specObj)
				errors = append(errors, err)
				continue specloop
			}

			// we only validate namespace level resources
			if ns != "" {
				if name == "" || kind == "" {
					err := fmt.Errorf("error: GetSpecNameKindNamepace returned values with Spec Name: [%s], Kind: [%s], Namespace: [%s], in local Context (NS): [%s], where some of the values are empty, so this object will be ignored", name, kind, ns, expectedRestoredAppContextNamespace)
					errors = append(errors, err)
					continue specloop
				}

				if kind == "StorageClass" || kind == "VolumeSnapshot" {
					// we don't restore "StorageClass"s and "VolumeSnapshot"s (becuase we don't back them up)
					continue specloop
				}

				if len(resourceTypesFilter) > 0 && !Contains(resourceTypesFilter, kind) {
					log.Infof("kind: [%s] is not in resourceTypesFilter [%v], so object (name: [%s], kind: [%s], namespace: [%s]) in expectedRestoredAppContext [%s] will not be checked for in restore [%s]", kind, resourceTypesFilter, name, kind, ns, expectedRestoredAppContextNamespace, restoreName)
					continue specloop
				}

				for _, restoredObj := range restoredObjectsInNS {
					if name == restoredObj.Name &&
						kind == restoredObj.Kind {
						log.Infof("object (name: [%s], GVK: [%s,%s,%s], namespace: [%s]) was found in restore [%s], as expected by presence in expectedRestoredAppContext [%s]", restoredObj.Name, restoredObj.Group, restoredObj.Version, restoredObj.Kind, restoredObj.Namespace, restoreName, expectedRestoredAppContextNamespace)

						if restoredObj.Status.Status != api.RestoreInfo_StatusInfo_Success /*Can this also be partialsuccess?*/ {
							if restoredObj.Status.Status == api.RestoreInfo_StatusInfo_Retained {
								if theRestore.ReplacePolicy != api.ReplacePolicy_Retain {
									err := fmt.Errorf("object (name: [%s], kind: [%s], namespace: [%s]) was found in the restore [%s] (as expected by presence in expectedRestoredAppContext [%s]), but status was [Retained], with reason [%s], despite the replace policy being [%s]", name, kind, ns, restoreName, expectedRestoredAppContextNamespace, restoredObj.Status.Reason, theRestore.ReplacePolicy)
									errors = append(errors, err)
								}
							} else {
								err := fmt.Errorf("object (name: [%s], kind: [%s], namespace: [%s]) was found in the restore [%s] (as expected by presence in expectedRestoredAppContext [%s]), but status was [%s], with reason [%s]", name, kind, ns, restoreName, expectedRestoredAppContextNamespace, restoredObj.Status.Status, restoredObj.Status.Reason)
								errors = append(errors, err)
							}
						}

						if k8s, ok := Inst().S.(*k8s.K8s); ok {
							_, err := k8s.GetUpdatedSpec(specObj)
							if err == nil {
								log.Infof("object (name: [%s], kind: [%s], namespace: [%s]) found in the restore [%s] was also present on the cluster/namespace [%s]", name, kind, ns, restoreName, expectedRestoredAppContextNamespace)
							} else {
								err := fmt.Errorf("prsence of object (name: [%s], kind: [%s], namespace: [%s]) found in the restore [%s] on the cluster/namespace [%s] could not be verified as scheduler is not K8s", name, kind, ns, restoreName, expectedRestoredAppContextNamespace)
								errors = append(errors, err)
							}
						} else {
							err := fmt.Errorf("prsence of object (name: [%s], kind: [%s], namespace: [%s]) found in the restore [%s] on the cluster/namespace [%s] could not be verified as scheduler is not K8s", name, kind, ns, restoreName, expectedRestoredAppContextNamespace)
							errors = append(errors, err)
						}

						continue specloop
					}
				}

				// The following error means that something was NOT backed up or restored,
				// OR it wasn't supposed to be either backed up or restored, and we forgot to exclude the check.
				err := fmt.Errorf("object (name: [%s], kind: [%s], namespace: [%s]) is not present in restore [%s], but was expected by it's presence in expectedRestoredAppContext [%s]", name, kind, ns, restoreName, expectedRestoredAppContextNamespace)
				errors = append(errors, err)

				if kind == "PersistentVolumeClaim" {
					err := fmt.Errorf("object (name: [%s], namespace: [%s]) is not present in restore [%s] is a PersistentVolumeClaim. Hence verification of existence of the corresponding volumes can't be done", name, ns, restoreName)
					errors = append(errors, err)
				}

				continue specloop
			}
		}

		// VALIDATION OF VOLUMES
		log.InfoD("Validating Restored Volumes for the namespace (restoredAppContext) [%s] in restore [%s]", expectedRestoredAppContextNamespace, restoreName)

		// Collect all volumes belonging to a context
		log.Infof("getting the volumes bounded to the PVCs in the namespace (restoredAppContext) [%s] in restore [%s]", expectedRestoredAppContextNamespace, restoreName)
		actualVolumeMap := make(map[string]*volume.Volume)
		actualRestoredVolumes, err := Inst().S.GetVolumes(expectedRestoredAppContext)
		if err != nil {
			err := fmt.Errorf("error getting volumes for namespace (expectedRestoredAppContext) [%s], hence skipping volume validation. Error in Inst().S.GetVolumes: [%v]", expectedRestoredAppContextNamespace, err)
			errors = append(errors, err)
			continue
		}
		for _, restoredVol := range actualRestoredVolumes {
			actualVolumeMap[restoredVol.ID] = restoredVol
		}
		log.Infof("volumes bounded to the PVCs in the context [%s] are [%+v]", expectedRestoredAppContextNamespace, actualRestoredVolumes)

		// looping over the list of volumes that PX-Backup says it restored, to run some checks
		for _, restoredVolInfo := range apparentlyRestoredVolumes {
			if namespaceMappings[restoredVolInfo.SourceNamespace] == expectedRestoredAppContextNamespace {
				if restoredVolInfo.Status.Status != api.RestoreInfo_StatusInfo_Success /*Can this also be partialsuccess?*/ {
					err := fmt.Errorf("in restore [%s], the status of the restored volume [%s] was not Success. It was [%s] with reason [%s]", restoreName, restoredVolInfo.RestoreVolume, restoredVolInfo.Status.Status, restoredVolInfo.Status.Reason)
					errors = append(errors, err)
					continue
				}

				var actualVol *volume.Volume
				var ok bool
				if actualVol, ok = actualVolumeMap[restoredVolInfo.RestoreVolume]; !ok {
					err := fmt.Errorf("in restore [%s], said restored volume [%s] cannot be found in the actual cluster [%s]", restoreName, restoredVolInfo.RestoreVolume, expectedRestoredAppContextNamespace)
					errors = append(errors, err)
					continue
				}

				if actualVol.Name != restoredVolInfo.Pvc {
					err := fmt.Errorf("in restore [%s], for restored volume [%s], PVC used is given as [%s], but actual PVC used was found to be [%s]", restoreName, restoredVolInfo.RestoreVolume, restoredVolInfo.Pvc, actualVol.Name)
					errors = append(errors, err)
				}

				var expectedVolumeDriver string
				if os.Getenv("BACKUP_TYPE") == "Generic" {
					expectedVolumeDriver = "kdmp"
				} else {
					expectedVolumeDriver = Inst().V.String()
				}

				if restoredVolInfo.DriverName != expectedVolumeDriver {
					err := fmt.Errorf("in restore [%s], for restored volume [%s], volume driver actually used is given as [%s], but expected is [%s]", restoreName, restoredVolInfo.RestoreVolume, restoredVolInfo.DriverName, expectedVolumeDriver)
					errors = append(errors, err)
				}
			}
		}

		// VALIDATE APPLICATIONS
		log.InfoD("Validate applications in restored namespace [%s] due to restore [%s]", expectedRestoredAppContextNamespace, restoreName)
		errorChan := make(chan error, errorChannelSize)
		ValidateContext(expectedRestoredAppContext, &errorChan)
		for err := range errorChan {
			errors = append(errors, err)
		}
	}

	errStrings := make([]string, 0)
	for _, err := range errors {
		if err != nil {
			errStrings = append(errStrings, err.Error())
		}
	}

	if len(errStrings) > 0 {
		return fmt.Errorf("ValidateRestore Errors: {%s}", strings.Join(errStrings, "}\n{"))
	} else {
		return nil
	}
}

// CloneAppContextAndTransformWithMappings clones an appContext and transforms it according to the maps provided. Set `forRestore` to true when the transformation is for namespaces restored by px-backup. To be used after switching to k8s context (cluster) which has the restored namespace.
func CloneAppContextAndTransformWithMappings(appContext *scheduler.Context, namespaceMapping map[string]string, storageClassMapping map[string]string, forRestore bool) (*scheduler.Context, error) {
	appContextNamespace := appContext.ScheduleOptions.Namespace
	log.Infof("TransformAppContextWithMappings of appContext [%s] with namespace mapping [%v] and storage Class Mapping [%v]", appContextNamespace, namespaceMapping, storageClassMapping)

	restoreAppContext := *appContext
	var errors []error

	// TODO: remove workaround in future.
	allStorageClassMappingsPresent := true

	specObjects := make([]interface{}, 0)
	for _, appSpecOrig := range appContext.App.SpecList {
		if forRestore {
			// if we are transforming to obtain a restored specs, VolumeSnapshot should be ignored
			if obj, ok := appSpecOrig.(*snapv1.VolumeSnapshot); ok {
				log.Infof("TransformAppContextWithMappings is for restore contexts, ignoring transformation of 'VolumeSnapshot' [%s] in appContext [%s]", obj.Metadata.Name, appContextNamespace)
				continue
			} else if obj, ok := appSpecOrig.(*storageapi.StorageClass); ok {
				log.Infof("TransformAppContextWithMappings is for restore contexts, ignoring transformation of 'StorageClass' [%s] in appContext [%s]", obj.Name, appContextNamespace)
				continue
			}
		}

		appSpec, err := CloneSpec(appSpecOrig) //clone spec to create "restore" specs
		if err != nil {
			err := fmt.Errorf("failed to clone spec: '%v'. Err: %v", appSpecOrig, err)
			errors = append(errors, err)
			continue
		}
		if forRestore {
			err = TransformToRestoredSpec(appSpec, storageClassMapping)
			if err != nil {
				err := fmt.Errorf("failed to TransformToRestoredSpec for %v, with sc map %s. Err: %v", appSpec, storageClassMapping, err)
				errors = append(errors, err)
				continue
			}
		}
		err = UpdateNamespace(appSpec, namespaceMapping)
		if err != nil {
			err := fmt.Errorf("failed to Update the namespace for %v, with ns map %s. Err: %v", appSpec, namespaceMapping, err)
			errors = append(errors, err)
			continue
		}
		specObjects = append(specObjects, appSpec)

		// TODO: remove workaround in future.
		if specObj, ok := appSpecOrig.(*corev1.PersistentVolumeClaim); ok {
			if _, ok := storageClassMapping[*specObj.Spec.StorageClassName]; !ok {
				allStorageClassMappingsPresent = false
			}
		}
	}

	errStrings := make([]string, 0)
	for _, err := range errors {
		if err != nil {
			errStrings = append(errStrings, err.Error())
		}
	}

	if len(errStrings) > 0 {
		return nil, fmt.Errorf("TransformAppContextWithMappings Errors: {%s}", strings.Join(errStrings, "}\n{"))
	}

	app := *appContext.App
	app.SpecList = specObjects
	restoreAppContext.App = &app

	// `CreateScheduleOptions` must be used in order to make it appear as though we scheduled it (rather than it being restored) in order to prove equivalency between scheduling and restoration.
	var options scheduler.ScheduleOptions
	if namespace, ok := namespaceMapping[appContextNamespace]; ok {
		options = CreateScheduleOptions(namespace)
	} else {
		options = CreateScheduleOptions(appContextNamespace)
	}
	restoreAppContext.ScheduleOptions = options

	// TODO: remove workaround in future.
	if !allStorageClassMappingsPresent {
		restoreAppContext.SkipVolumeValidation = true
	}

	return &restoreAppContext, nil
}

// TransformToRestoredSpec transforms a given spec to one expected in case of restoration by px-backup. An error is retuned if any transformation fails. specs with no need for transformation are ignored.
func TransformToRestoredSpec(spec interface{}, storageClassMapping map[string]string) error {
	if specObj, ok := spec.(*corev1.PersistentVolumeClaim); ok {
		if sc, ok := storageClassMapping[*specObj.Spec.StorageClassName]; ok {
			*specObj.Spec.StorageClassName = sc
		}
		return nil
	}

	return nil
}

// IsBackupLocationPresent checks whether the backup location is present or not
func IsBackupLocationPresent(bkpLocation string, ctx context.Context, orgID string) (bool, error) {
	backupLocationNames := make([]string, 0)
	backupLocationEnumerateRequest := &api.BackupLocationEnumerateRequest{
		OrgId: orgID,
	}
	response, err := Inst().Backup.EnumerateBackupLocation(ctx, backupLocationEnumerateRequest)
	if err != nil {
		return false, err
	}

	for _, backupLocationObj := range response.GetBackupLocations() {
		backupLocationNames = append(backupLocationNames, backupLocationObj.GetName())
		if backupLocationObj.GetName() == bkpLocation {
			log.Infof("Backup location [%s] is present", bkpLocation)
			return true, nil
		}
	}
	log.Infof("Backup locations fetched - %s", backupLocationNames)
	return false, nil
}

// IsCloudCredPresent checks whether the Cloud Cred is present or not
func IsCloudCredPresent(cloudCredName string, ctx context.Context, orgID string) (bool, error) {
	cloudCredEnumerateRequest := &api.CloudCredentialEnumerateRequest{
		OrgId:          orgID,
		IncludeSecrets: false,
	}
	cloudCredObjs, err := Inst().Backup.EnumerateCloudCredential(ctx, cloudCredEnumerateRequest)
	if err != nil {
		return false, err
	}
	for _, cloudCredObj := range cloudCredObjs.GetCloudCredentials() {
		if cloudCredObj.GetName() == cloudCredName {
			log.Infof("Cloud Credential [%s] is present", cloudCredName)
			return true, nil
		}
	}
	return false, nil
}

// CreateCustomRestoreWithPVCs function can be used to deploy custom deployment with it's PVCs. It cannot be used for any other resource type.
func CreateCustomRestoreWithPVCs(restoreName string, backupName string, namespaceMapping map[string]string, clusterName string,
	orgID string, ctx context.Context, storageClassMapping map[string]string, namespace string) (deploymentName string, err error) {

	var bkpUid string
	var newResources []*api.ResourceInfo
	var options metav1.ListOptions
	var deploymentPvcMap = make(map[string][]string)
	backupDriver := Inst().Backup
	log.Infof("Getting the UID of the backup needed to be restored")
	bkpUid, err = backupDriver.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return "", fmt.Errorf("unable to get backup UID for %v with error %v", backupName, err)
	}
	deploymentList, err := apps.Instance().ListDeployments(namespace, options)
	if err != nil {
		return "", fmt.Errorf("unable to list the deployments in namespace %v with error %v", namespace, err)
	}
	if len(deploymentList.Items) == 0 {
		return "", fmt.Errorf("deployment list is null")
	}
	deployments := deploymentList.Items
	for _, deployment := range deployments {
		var pvcs []string
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			pvcName := vol.PersistentVolumeClaim.ClaimName
			pvcs = append(pvcs, pvcName)
		}
		deploymentPvcMap[deployment.Name] = pvcs
	}
	// select a random index from the slice of deployment names to be restored
	randomIndex := rand.Intn(len(deployments))
	deployment := deployments[randomIndex]
	log.Infof("selected deployment %v", deployment.Name)
	pvcs, exists := deploymentPvcMap[deployment.Name]
	if !exists {
		return "", fmt.Errorf("deploymentName %v not found in the deploymentPvcMap", deployment.Name)
	}
	deploymentStruct := &api.ResourceInfo{
		Version:   "v1",
		Group:     "apps",
		Kind:      "Deployment",
		Name:      deployment.Name,
		Namespace: namespace,
	}
	pvcsStructs := make([]*api.ResourceInfo, len(pvcs))
	for i, pvcName := range pvcs {
		pvcStruct := &api.ResourceInfo{
			Version:   "v1",
			Group:     "core",
			Kind:      "PersistentVolumeClaim",
			Name:      pvcName,
			Namespace: namespace,
		}
		pvcsStructs[i] = pvcStruct
	}
	newResources = append([]*api.ResourceInfo{deploymentStruct}, pvcsStructs...)
	createRestoreReq := &api.RestoreCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  restoreName,
			OrgId: orgID,
		},
		Backup:              backupName,
		Cluster:             clusterName,
		NamespaceMapping:    namespaceMapping,
		StorageClassMapping: storageClassMapping,
		BackupRef: &api.ObjectRef{
			Name: backupName,
			Uid:  bkpUid,
		},
		IncludeResources: newResources,
	}
	_, err = backupDriver.CreateRestore(ctx, createRestoreReq)
	if err != nil {
		return "", fmt.Errorf("fail to create restore with createrestore req %v and error %v", createRestoreReq, err)
	}
	err = restoreSuccessCheck(restoreName, orgID, maxWaitPeriodForRestoreCompletionInMinute*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return "", fmt.Errorf("fail to create restore %v with error %v", restoreName, err)
	}
	return deployment.Name, nil
}

// GetOrdinalScheduleBackupName returns the name of the schedule backup at the specified ordinal position for the given schedule
func GetOrdinalScheduleBackupName(ctx context.Context, scheduleName string, ordinal int, orgID string) (string, error) {
	if ordinal < 1 {
		return "", fmt.Errorf("the provided ordinal value [%d] for schedule backups with schedule name [%s] is invalid. valid values range from 1", ordinal, scheduleName)
	}
	allScheduleBackupNames, err := Inst().Backup.GetAllScheduleBackupNames(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	if len(allScheduleBackupNames) == 0 {
		return "", fmt.Errorf("no backups were found for the schedule [%s]", scheduleName)
	}
	if ordinal > len(allScheduleBackupNames) {
		return "", fmt.Errorf("schedule backups with schedule name [%s] have not been created up to the provided ordinal value [%d]", scheduleName, ordinal)
	}
	return allScheduleBackupNames[ordinal-1], nil
}

// GetFirstScheduleBackupName returns the name of the first schedule backup for the given schedule
func GetFirstScheduleBackupName(ctx context.Context, scheduleName string, orgID string) (string, error) {
	allScheduleBackupNames, err := Inst().Backup.GetAllScheduleBackupNames(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	if len(allScheduleBackupNames) == 0 {
		return "", fmt.Errorf("no backups found for schedule %s", scheduleName)
	}
	return allScheduleBackupNames[0], nil
}

// GetLatestScheduleBackupName returns the name of the latest schedule backup for the given schedule
func GetLatestScheduleBackupName(ctx context.Context, scheduleName string, orgID string) (string, error) {
	allScheduleBackupNames, err := Inst().Backup.GetAllScheduleBackupNames(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	if len(allScheduleBackupNames) == 0 {
		return "", fmt.Errorf("no backups found for schedule %s", scheduleName)
	}
	return allScheduleBackupNames[len(allScheduleBackupNames)-1], nil
}

// GetOrdinalScheduleBackupUID returns the uid of the schedule backup at the specified ordinal position for the given schedule
func GetOrdinalScheduleBackupUID(ctx context.Context, scheduleName string, ordinal int, orgID string) (string, error) {
	if ordinal < 1 {
		return "", fmt.Errorf("the provided ordinal value [%d] for schedule backups with schedule name [%s] is invalid. valid values range from 1", ordinal, scheduleName)
	}
	allScheduleBackupUids, err := Inst().Backup.GetAllScheduleBackupUIDs(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	if len(allScheduleBackupUids) == 0 {
		return "", fmt.Errorf("no backups were found for the schedule [%s]", scheduleName)
	}
	if ordinal > len(allScheduleBackupUids) {
		return "", fmt.Errorf("schedule backups with schedule name [%s] have not been created up to the provided ordinal value [%d]", scheduleName, ordinal)
	}
	return allScheduleBackupUids[ordinal-1], nil
}

// GetFirstScheduleBackupUID returns the uid of the first schedule backup for the given schedule
func GetFirstScheduleBackupUID(ctx context.Context, scheduleName string, orgID string) (string, error) {
	allScheduleBackupUids, err := Inst().Backup.GetAllScheduleBackupUIDs(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	if len(allScheduleBackupUids) == 0 {
		return "", fmt.Errorf("no backups found for schedule %s", scheduleName)
	}
	return allScheduleBackupUids[0], nil
}

// GetLatestScheduleBackupUID returns the uid of the latest schedule backup for the given schedule
func GetLatestScheduleBackupUID(ctx context.Context, scheduleName string, orgID string) (string, error) {
	allScheduleBackupUids, err := Inst().Backup.GetAllScheduleBackupUIDs(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	if len(allScheduleBackupUids) == 0 {
		return "", fmt.Errorf("no backups found for schedule %s", scheduleName)
	}
	return allScheduleBackupUids[len(allScheduleBackupUids)-1], nil
}

// IsPresent verifies if the given data is present in slice of data
func IsPresent(dataSlice interface{}, data interface{}) bool {
	s := reflect.ValueOf(dataSlice)
	for i := 0; i < s.Len(); i++ {
		if s.Index(i).Interface() == data {
			return true
		}
	}
	return false
}

func DeleteBackupAndWait(backupName string, ctx context.Context) error {
	backupDriver := Inst().Backup
	backupEnumerateReq := &api.BackupEnumerateRequest{
		OrgId: orgID,
	}

	backupDeletionSuccessCheck := func() (interface{}, bool, error) {
		currentBackups, err := backupDriver.EnumerateBackup(ctx, backupEnumerateReq)
		if err != nil {
			return "", true, err
		}
		for _, backupObject := range currentBackups.GetBackups() {
			if backupObject.Name == backupName {
				return "", true, fmt.Errorf("backupObject [%s] is not yet deleted", backupObject.Name)
			}
		}
		return "", false, nil
	}
	_, err := task.DoRetryWithTimeout(backupDeletionSuccessCheck, backupDeleteTimeout, backupDeleteRetryTime)
	return err
}

// GetPxBackupVersion return the version of Px Backup as a VersionInfo struct
func GetPxBackupVersion() (*api.VersionInfo, error) {
	ctx, err := backup.GetAdminCtxFromSecret()
	if err != nil {
		return nil, err
	}
	versionResponse, err := Inst().Backup.GetPxBackupVersion(ctx, &api.VersionGetRequest{})
	if err != nil {
		return nil, err
	}
	backupVersion := versionResponse.GetVersion()
	return backupVersion, nil
}

// GetPxBackupVersionString returns the version of Px Backup like 2.4.0-e85b680
func GetPxBackupVersionString() (string, error) {
	backupVersion, err := GetPxBackupVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.%s-%s", backupVersion.GetMajor(), backupVersion.GetMinor(), backupVersion.GetPatch(), backupVersion.GetGitCommit()), nil
}

// GetPxBackupVersionSemVer returns the version of Px Backup in semver format like 2.4.0
func GetPxBackupVersionSemVer() (string, error) {
	backupVersion, err := GetPxBackupVersion()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.%s", backupVersion.GetMajor(), backupVersion.GetMinor(), backupVersion.GetPatch()), nil
}

// GetPxBackupBuildDate returns the Px Backup build date
func GetPxBackupBuildDate() (string, error) {
	ctx, err := backup.GetAdminCtxFromSecret()
	if err != nil {
		return "", err
	}
	versionResponse, err := Inst().Backup.GetPxBackupVersion(ctx, &api.VersionGetRequest{})
	if err != nil {
		return "", err
	}
	backupVersion := versionResponse.GetVersion()
	return backupVersion.GetBuildDate(), nil
}

// UpgradePxBackup will perform the upgrade tasks for Px Backup to the version passed as string
// Eg: versionToUpgrade := "2.4.0"
func UpgradePxBackup(versionToUpgrade string) error {
	var cmd string

	// Compare and validate the upgrade path
	currentBackupVersionString, err := GetPxBackupVersionSemVer()
	if err != nil {
		return err
	}
	currentBackupVersion, err := version.NewSemver(currentBackupVersionString)
	if err != nil {
		return err
	}
	versionToUpgradeSemVer, err := version.NewSemver(versionToUpgrade)
	if err != nil {
		return err
	}

	if currentBackupVersion.GreaterThanOrEqual(versionToUpgradeSemVer) {
		return fmt.Errorf("px backup cannot be upgraded from version [%s] to version [%s]", currentBackupVersion.String(), versionToUpgradeSemVer.String())
	} else {
		log.InfoD("Upgrade path chosen (%s) ---> (%s)", currentBackupVersionString, versionToUpgrade)
	}

	// Getting Px Backup Namespace
	pxBackupNamespace, err := backup.GetPxBackupNamespace()
	if err != nil {
		return err
	}

	// Delete the pxcentral-post-install-hook job is it exists
	allJobs, err := batch.Instance().ListAllJobs(pxBackupNamespace, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(allJobs.Items) > 0 {
		log.Infof("List of all the jobs in Px Backup Namespace [%s] - ", pxBackupNamespace)
		for _, job := range allJobs.Items {
			log.Infof(job.Name)
		}

		for _, job := range allJobs.Items {
			if strings.Contains(job.Name, pxCentralPostInstallHookJobName) {
				err = deleteJobAndWait(job)
				if err != nil {
					return err
				}
			}
		}
	} else {
		log.Infof("%s job not found", pxCentralPostInstallHookJobName)
	}

	// Get storage class using for px-backup deployment
	statefulSet, err := apps.Instance().GetStatefulSet(mongodbStatefulset, pxBackupNamespace)
	if err != nil {
		return err
	}
	pvcs, err := apps.Instance().GetPVCsForStatefulSet(statefulSet)
	if err != nil {
		return err
	}
	storageClassName := pvcs.Items[0].Spec.StorageClassName

	// Get the tarball required for helm upgrade
	helmBranch, isPresent := os.LookupEnv("PX_BACKUP_HELM_REPO_BRANCH")
	if !isPresent {
		helmBranch = defaultPxBackupHelmBranch
	}
	cmd = fmt.Sprintf("curl -O  https://raw.githubusercontent.com/portworx/helm/%s/stable/px-central-%s.tgz", helmBranch, versionToUpgrade)
	log.Infof("curl command to get tarball: %v ", cmd)
	output, _, err := osutils.ExecShell(cmd)
	if err != nil {
		return fmt.Errorf("error downloading of tarball: %v", err)
	}
	log.Infof("Terminal output: %s", output)

	// Checking if all pods are healthy before upgrade
	err = ValidateAllPodsInPxBackupNamespace()
	if err != nil {
		return err
	}

	// Execute helm upgrade using cmd
	log.Infof("Upgrading Px-Backup version from %s to %s", currentBackupVersionString, versionToUpgrade)
	cmd = fmt.Sprintf("helm upgrade px-central px-central-%s.tgz --namespace %s --version %s --set persistentStorage.enabled=true,persistentStorage.storageClassName=\"%s\",pxbackup.enabled=true",
		versionToUpgrade, pxBackupNamespace, versionToUpgrade, *storageClassName)
	log.Infof("helm command: %v ", cmd)

	pxBackupUpgradeStartTime := time.Now()

	output, _, err = osutils.ExecShell(cmd)
	if err != nil {
		return fmt.Errorf("upgrade failed with error: %v", err)
	}
	log.Infof("Terminal output: %s", output)

	// Collect mongoDB logs right after the command
	ginkgoTest := CurrentGinkgoTestDescription()
	testCaseName := fmt.Sprintf("%s-start", ginkgoTest.FullTestText)
	CollectMongoDBLogs(testCaseName)

	// Wait for post install hook job to be completed
	postInstallHookJobCompletedCheck := func() (interface{}, bool, error) {
		job, err := batch.Instance().GetJob(pxCentralPostInstallHookJobName, pxBackupNamespace)
		if err != nil {
			return "", true, err
		}
		if job.Status.Succeeded > 0 {
			log.Infof("Status of job %s after completion - "+
				"\nactive count - %d"+
				"\nsucceeded count - %d"+
				"\nfailed count - %d\n", job.Name, job.Status.Active, job.Status.Succeeded, job.Status.Failed)
			return "", false, nil
		}
		return "", true, fmt.Errorf("status of job %s not yet in desired state - "+
			"\nactive count - %d"+
			"\nsucceeded count - %d"+
			"\nfailed count - %d\n", job.Name, job.Status.Active, job.Status.Succeeded, job.Status.Failed)
	}
	_, err = task.DoRetryWithTimeout(postInstallHookJobCompletedCheck, 10*time.Minute, 30*time.Second)
	if err != nil {
		return err
	}

	// Collect mongoDB logs once the postInstallHook is completed
	ginkgoTest = CurrentGinkgoTestDescription()
	testCaseName = fmt.Sprintf("%s-end", ginkgoTest.FullTestText)
	CollectMongoDBLogs(testCaseName)

	pxBackupUpgradeEndTime := time.Now()
	pxBackupUpgradeDuration := pxBackupUpgradeEndTime.Sub(pxBackupUpgradeStartTime)
	log.InfoD("Time taken for Px-Backup upgrade to complete: %02d:%02d:%02d hh:mm:ss", int(pxBackupUpgradeDuration.Hours()), int(pxBackupUpgradeDuration.Minutes())%60, int(pxBackupUpgradeDuration.Seconds())%60)

	// Checking if all pods are running
	err = ValidateAllPodsInPxBackupNamespace()
	if err != nil {
		return err
	}

	postUpgradeVersion, err := GetPxBackupVersionSemVer()
	if err != nil {
		return err
	}
	if !strings.EqualFold(postUpgradeVersion, versionToUpgrade) {
		return fmt.Errorf("expected version after upgrade was %s but got %s", versionToUpgrade, postUpgradeVersion)
	}
	log.InfoD("Px-Backup upgrade from %s to %s is complete", currentBackupVersionString, postUpgradeVersion)
	return nil
}

// deleteJobAndWait waits for the provided job to be deleted
func deleteJobAndWait(job batchv1.Job) error {
	t := func() (interface{}, bool, error) {
		err := batch.Instance().DeleteJob(job.Name, job.Namespace)

		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return "", false, nil
			}
			return "", false, err
		}
		return "", true, fmt.Errorf("job %s not deleted", job.Name)
	}

	_, err := task.DoRetryWithTimeout(t, jobDeleteTimeout, jobDeleteRetryTime)
	if err != nil {
		return err
	}
	log.Infof("job %s deleted", job.Name)
	return nil
}

func ValidateAllPodsInPxBackupNamespace() error {
	pxBackupNamespace, err := backup.GetPxBackupNamespace()
	if err != nil {
		return err
	}
	for _, label := range backupPodLabels {
		allPods, err := core.Instance().GetPods(pxBackupNamespace, label)
		if err != nil {
			return err
		}
		for _, pod := range allPods.Items {
			log.Infof("Checking status for pod - %s", pod.GetName())
			err = core.Instance().ValidatePod(&pod, 5*time.Minute, 30*time.Second)
			if err != nil {
				// Collect mongoDB logs right after the command
				ginkgoTest := CurrentGinkgoTestDescription()
				testCaseName := ginkgoTest.FullTestText
				matches := regexp.MustCompile(`\{([^}]+)\}`).FindStringSubmatch(ginkgoTest.FullTestText)
				if len(matches) > 1 {
					testCaseName = fmt.Sprintf("%s-error-%s", matches[1], label)
				}
				CollectLogsFromPods(testCaseName, label, pxBackupNamespace, pod.GetName())
				return err
			}
		}
	}
	err = IsMongoDBReady()
	return err
}

// getStorkImageVersion returns current stork image version.
func getStorkImageVersion() (string, error) {
	storkDeploymentNamespace, err := k8sutils.GetStorkPodNamespace()
	if err != nil {
		return "", err
	}
	storkDeployment, err := apps.Instance().GetDeployment(storkDeploymentName, storkDeploymentNamespace)
	if err != nil {
		return "", err
	}
	storkImage := storkDeployment.Spec.Template.Spec.Containers[0].Image
	storkImageVersion := strings.Split(storkImage, ":")[len(strings.Split(storkImage, ":"))-1]
	return storkImageVersion, nil
}

// upgradeStorkVersion upgrades the stork to the provided version.
func upgradeStorkVersion(storkImageToUpgrade string) error {
	var finalImageToUpgrade string
	var postUpgradeStorkImageVersionStr string
	storkDeploymentNamespace, err := k8sutils.GetStorkPodNamespace()
	if err != nil {
		return err
	}
	currentStorkImageStr, err := getStorkImageVersion()
	if err != nil {
		return err
	}
	currentStorkVersion, err := version.NewSemver(currentStorkImageStr)
	if err != nil {
		return err
	}
	storkImageVersionToUpgrade, err := version.NewSemver(storkImageToUpgrade)
	if err != nil {
		return err
	}

	log.Infof("Current stork version : %s", currentStorkVersion)
	log.Infof("Upgrading stork version to : %s", storkImageVersionToUpgrade)

	if currentStorkVersion.GreaterThanOrEqual(storkImageVersionToUpgrade) {
		return fmt.Errorf("Cannot upgrade stork version from %s to %s as the current version is higher than the provided version", currentStorkVersion, storkImageVersionToUpgrade)
	}
	internalDockerRegistry := os.Getenv("INTERNAL_DOCKER_REGISTRY")
	if internalDockerRegistry != "" {
		finalImageToUpgrade = fmt.Sprintf("%s/portworx/stork:%s", internalDockerRegistry, storkImageToUpgrade)
	} else {
		finalImageToUpgrade = fmt.Sprintf("docker.io/openstorage/stork:%s", storkImageToUpgrade)
	}
	isOpBased, _ := Inst().V.IsOperatorBasedInstall()
	if isOpBased {
		log.Infof("Operator based Portworx deployment, Upgrading stork via StorageCluster")
		storageSpec, err := Inst().V.GetDriver()
		if err != nil {
			return err
		}
		storageSpec.Spec.Stork.Image = finalImageToUpgrade
		_, err = operator.Instance().UpdateStorageCluster(storageSpec)
		if err != nil {
			return err
		}
	} else {
		log.Infof("Non-Operator based Portworx deployment, Upgrading stork via Deployment")
		storkDeployment, err := apps.Instance().GetDeployment(storkDeploymentName, storkDeploymentNamespace)
		if err != nil {
			return err
		}
		storkDeployment.Spec.Template.Spec.Containers[0].Image = finalImageToUpgrade
		_, err = apps.Instance().UpdateDeployment(storkDeployment)
		if err != nil {
			return err
		}
	}
	// Wait for upgrade request to go through before validating
	t := func() (interface{}, bool, error) {
		postUpgradeStorkImageVersionStr, err = getStorkImageVersion()
		if err != nil {
			return "", true, err
		}
		if !strings.EqualFold(postUpgradeStorkImageVersionStr, storkImageToUpgrade) {
			return "", true, fmt.Errorf("expected version after upgrade was %s but got %s", storkImageToUpgrade, postUpgradeStorkImageVersionStr)
		}
		return "", false, nil
	}
	_, err = task.DoRetryWithTimeout(t, 5*time.Minute, 30*time.Second)
	if err != nil {
		return err
	}

	// validate stork pods after upgrade
	updatedStorkDeployment, err := apps.Instance().GetDeployment(storkDeploymentName, storkDeploymentNamespace)
	if err != nil {
		return err
	}
	err = apps.Instance().ValidateDeployment(updatedStorkDeployment, storkPodReadyTimeout, podReadyRetryTime)
	if err != nil {
		return err
	}

	log.Infof("Successfully upgraded stork version from %v to %v", currentStorkImageStr, postUpgradeStorkImageVersionStr)
	return nil
}

// CreateBackupWithNamespaceLabel creates a backup with Namespace label and checks for success
func CreateBackupWithNamespaceLabel(backupName string, clusterName string, bkpLocation string, bkpLocationUID string,
	labelSelectors map[string]string, orgID string, uid string, preRuleName string, preRuleUid string, postRuleName string,
	postRuleUid string, namespaceLabel string, ctx context.Context) error {
	_, err := CreateBackupWithNamespaceLabelWithoutCheck(backupName, clusterName, bkpLocation, bkpLocationUID, labelSelectors, orgID, uid, preRuleName, preRuleUid, postRuleName, postRuleUid, namespaceLabel, ctx)
	if err != nil {
		return err
	}
	err = backupSuccessCheck(backupName, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Successfully created backup [%s] with namespace label [%s]", backupName, namespaceLabel)
	return nil
}

// CreateBackupWithNamespaceLabelWithValidation creates backup with namespace label, checks for success, and validates the backup.
func CreateBackupWithNamespaceLabelWithValidation(ctx context.Context, backupName string, clusterName string, bkpLocation string, bkpLocationUID string, scheduledAppContextsExpectedInBackup []*scheduler.Context, labelSelectors map[string]string, orgID string, uid string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string, namespaceLabel string) error {
	err := CreateBackupWithNamespaceLabel(backupName, clusterName, bkpLocation, bkpLocationUID, labelSelectors, orgID, uid, preRuleName, preRuleUid, postRuleName, postRuleUid, namespaceLabel, ctx)
	if err != nil {
		return err
	}
	return ValidateBackup(ctx, backupName, orgID, scheduledAppContextsExpectedInBackup, make([]string, 0))
}

// CreateScheduleBackupWithNamespaceLabel creates a schedule backup with namespace label and checks for success
func CreateScheduleBackupWithNamespaceLabel(scheduleName string, clusterName string, bkpLocation string, bkpLocationUID string, labelSelectors map[string]string, orgID string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string, namespaceLabel, schPolicyName string, schPolicyUID string, ctx context.Context) error {
	_, err := CreateScheduleBackupWithNamespaceLabelWithoutCheck(scheduleName, clusterName, bkpLocation, bkpLocationUID, labelSelectors, orgID, preRuleName, preRuleUid, postRuleName, postRuleUid, schPolicyName, schPolicyUID, namespaceLabel, ctx)
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Minute)
	firstScheduleBackupName, err := GetFirstScheduleBackupName(ctx, scheduleName, orgID)
	if err != nil {
		return err
	}
	log.InfoD("first schedule backup for schedule name [%s] is [%s]", scheduleName, firstScheduleBackupName)
	err = backupSuccessCheck(firstScheduleBackupName, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return err
	}
	log.Infof("Successfully created schedule backup [%s] with namespace label [%s]", firstScheduleBackupName, namespaceLabel)
	return nil
}

// CreateBackupWithNamespaceLabelWithoutCheck creates backup with namespace label filter without waiting for success
func CreateBackupWithNamespaceLabelWithoutCheck(backupName string, clusterName string, bkpLocation string, bkpLocationUID string,
	labelSelectors map[string]string, orgID string, uid string, preRuleName string, preRuleUid string, postRuleName string,
	postRuleUid string, namespaceLabel string, ctx context.Context) (*api.BackupInspectResponse, error) {

	backupDriver := Inst().Backup
	bkpCreateRequest := &api.BackupCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  backupName,
			OrgId: orgID,
		},
		BackupLocationRef: &api.ObjectRef{
			Name: bkpLocation,
			Uid:  bkpLocationUID,
		},
		Cluster:        clusterName,
		LabelSelectors: labelSelectors,
		ClusterRef: &api.ObjectRef{
			Name: clusterName,
			Uid:  uid,
		},
		PreExecRuleRef: &api.ObjectRef{
			Name: preRuleName,
			Uid:  preRuleUid,
		},
		PostExecRuleRef: &api.ObjectRef{
			Name: postRuleName,
			Uid:  postRuleUid,
		},
		NsLabelSelectors: namespaceLabel,
	}

	if strings.ToLower(os.Getenv("BACKUP_TYPE")) == "generic" {
		log.Infof("Detected generic backup type")
		bkpCreateRequest.BackupType = api.BackupCreateRequest_Generic
		var csiSnapshotClassName string
		var err error
		if csiSnapshotClassName, err = GetCsiSnapshotClassName(); err != nil {
			return nil, err
		}
		bkpCreateRequest.CsiSnapshotClassName = csiSnapshotClassName
	}
	_, err := backupDriver.CreateBackup(ctx, bkpCreateRequest)
	if err != nil {
		return nil, err
	}
	backupUid, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return nil, err
	}
	backupInspectRequest := &api.BackupInspectRequest{
		Name:  backupName,
		Uid:   backupUid,
		OrgId: orgID,
	}
	resp, err := backupDriver.InspectBackup(ctx, backupInspectRequest)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// CreateScheduleBackupWithNamespaceLabelWithoutCheck creates a schedule backup with namespace label filter without waiting for success
func CreateScheduleBackupWithNamespaceLabelWithoutCheck(scheduleName string, clusterName string, bkpLocation string, bkpLocationUID string, labelSelectors map[string]string, orgID string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string, schPolicyName string, schPolicyUID string, namespaceLabel string, ctx context.Context) (*api.BackupScheduleInspectResponse, error) {
	backupDriver := Inst().Backup
	bkpSchCreateRequest := &api.BackupScheduleCreateRequest{
		CreateMetadata: &api.CreateMetadata{
			Name:  scheduleName,
			OrgId: orgID,
		},
		SchedulePolicyRef: &api.ObjectRef{
			Name: schPolicyName,
			Uid:  schPolicyUID,
		},
		BackupLocationRef: &api.ObjectRef{
			Name: bkpLocation,
			Uid:  bkpLocationUID,
		},
		SchedulePolicy: schPolicyName,
		Cluster:        clusterName,
		LabelSelectors: labelSelectors,
		PreExecRuleRef: &api.ObjectRef{
			Name: preRuleName,
			Uid:  preRuleUid,
		},
		PostExecRuleRef: &api.ObjectRef{
			Name: postRuleName,
			Uid:  postRuleUid,
		},
		NsLabelSelectors: namespaceLabel,
	}
	_, err := backupDriver.CreateBackupSchedule(ctx, bkpSchCreateRequest)
	if err != nil {
		return nil, err
	}
	backupScheduleInspectRequest := &api.BackupScheduleInspectRequest{
		OrgId: orgID,
		Name:  scheduleName,
		Uid:   "",
	}
	resp, err := backupDriver.InspectBackupSchedule(ctx, backupScheduleInspectRequest)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// CreateScheduleBackupWithNamespaceLabelWithValidation creates a schedule backup with namespace label, checks for success of first (immediately triggered) backup, validates that backup and returns the name of that first scheduled backup
func CreateScheduleBackupWithNamespaceLabelWithValidation(ctx context.Context, scheduleName string, clusterName string, bkpLocation string, bkpLocationUID string, scheduledAppContextsExpectedInBackup []*scheduler.Context, labelSelectors map[string]string, orgID string, preRuleName string, preRuleUid string, postRuleName string, postRuleUid string, namespaceLabel string, schPolicyName string, schPolicyUID string) (string, error) {
	_, err := CreateScheduleBackupWithNamespaceLabelWithoutCheck(scheduleName, clusterName, bkpLocation, bkpLocationUID, labelSelectors, orgID, preRuleName, preRuleUid, postRuleName, postRuleUid, schPolicyName, schPolicyUID, namespaceLabel, ctx)
	if err != nil {
		return "", err
	}
	time.Sleep(1 * time.Minute)
	firstScheduleBackupName, err := GetFirstScheduleBackupName(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	log.InfoD("first schedule backup for schedule name [%s] is [%s]", scheduleName, firstScheduleBackupName)
	return firstScheduleBackupName, backupSuccessCheckWithValidation(ctx, firstScheduleBackupName, scheduledAppContextsExpectedInBackup, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second)
}

// suspendBackupSchedule will suspend backup schedule
func suspendBackupSchedule(backupScheduleName, schPolicyName, OrgID string, ctx context.Context) error {
	backupDriver := Inst().Backup
	backupScheduleUID, err := GetScheduleUID(backupScheduleName, orgID, ctx)
	if err != nil {
		return err
	}
	schPolicyUID, err := Inst().Backup.GetSchedulePolicyUid(orgID, ctx, schPolicyName)
	if err != nil {
		return err
	}
	bkpScheduleSuspendRequest := &api.BackupScheduleUpdateRequest{
		CreateMetadata: &api.CreateMetadata{Name: backupScheduleName, OrgId: OrgID, Uid: backupScheduleUID},
		Suspend:        true,
		SchedulePolicyRef: &api.ObjectRef{
			Name: schPolicyName,
			Uid:  schPolicyUID,
		},
	}
	_, err = backupDriver.UpdateBackupSchedule(ctx, bkpScheduleSuspendRequest)
	return err
}

// resumeBackupSchedule will resume backup schedule
func resumeBackupSchedule(backupScheduleName, schPolicyName, OrgID string, ctx context.Context) error {
	backupDriver := Inst().Backup
	backupScheduleUID, err := GetScheduleUID(backupScheduleName, orgID, ctx)
	if err != nil {
		return err
	}
	schPolicyUID, err := Inst().Backup.GetSchedulePolicyUid(orgID, ctx, schPolicyName)
	if err != nil {
		return err
	}
	bkpScheduleSuspendRequest := &api.BackupScheduleUpdateRequest{
		CreateMetadata: &api.CreateMetadata{Name: backupScheduleName, OrgId: OrgID, Uid: backupScheduleUID},
		Suspend:        false,
		SchedulePolicyRef: &api.ObjectRef{
			Name: schPolicyName,
			Uid:  schPolicyUID,
		},
	}
	_, err = backupDriver.UpdateBackupSchedule(ctx, bkpScheduleSuspendRequest)
	return err
}

// NamespaceLabelBackupSuccessCheck verifies if the labeled namespaces are backed up and checks for labels applied to backups
func NamespaceLabelBackupSuccessCheck(backupName string, ctx context.Context, listOfLabelledNamespaces []string, namespaceLabel string) error {
	backupDriver := Inst().Backup
	log.Infof("Getting the Uid of backup %v", backupName)
	backupUid, err := backupDriver.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return err
	}
	backupInspectRequest := &api.BackupInspectRequest{
		Name:  backupName,
		Uid:   backupUid,
		OrgId: orgID,
	}
	resp, err := backupDriver.InspectBackup(ctx, backupInspectRequest)
	if err != nil {
		return err
	}
	namespaceList := resp.GetBackup().GetNamespaces()
	log.Infof("The list of namespaces backed up are %v", namespaceList)
	if !AreStringSlicesEqual(namespaceList, listOfLabelledNamespaces) {
		return fmt.Errorf("list of namespaces backed up are %v which is not same as expected %v", namespaceList, listOfLabelledNamespaces)
	}
	backupLabels := resp.GetBackup().GetNsLabelSelectors()
	log.Infof("The list of labels applied to backup are %v", backupLabels)
	expectedLabels := strings.Split(namespaceLabel, ",")
	actualLabels := strings.Split(backupLabels, ",")
	AreStringSlicesEqual(expectedLabels, actualLabels)
	if !AreStringSlicesEqual(expectedLabels, actualLabels) {
		return fmt.Errorf("labels applied to backup are %v which is not same as expected %v", actualLabels, expectedLabels)
	}
	return nil
}

// AddLabelsToMultipleNamespaces add labels to multiple namespace
func AddLabelsToMultipleNamespaces(labels map[string]string, namespaces []string) error {
	for _, namespace := range namespaces {
		err := Inst().S.AddNamespaceLabel(namespace, labels)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteLabelsFromMultipleNamespaces delete labels from multiple namespace
func DeleteLabelsFromMultipleNamespaces(labels map[string]string, namespaces []string) error {
	for _, namespace := range namespaces {
		err := Inst().S.RemoveNamespaceLabel(namespace, labels)
		if err != nil {
			return err
		}
	}
	return nil
}

// GenerateRandomLabels creates random label
func GenerateRandomLabels(number int) map[string]string {
	labels := make(map[string]string)
	randomString := uuid.New()
	for i := 0; i < number; i++ {
		key := fmt.Sprintf("%v-%v", i, randomString)
		value := randomString
		labels[key] = value
	}
	return labels
}

// MapToKeyValueString converts a map of string keys and value to a comma separated string of "key=value"
func MapToKeyValueString(m map[string]string) string {
	var pairs []string
	for k, v := range m {
		pairs = append(pairs, k+"="+v)
	}
	return strings.Join(pairs, ",")
}

// VerifyLicenseConsumedCount verifies the consumed license count for px-backup
func VerifyLicenseConsumedCount(ctx context.Context, OrgId string, expectedLicenseConsumedCount int64) error {
	licenseInspectRequestObject := &api.LicenseInspectRequest{
		OrgId: OrgId,
	}
	licenseCountCheck := func() (interface{}, bool, error) {
		licenseInspectResponse, err := Inst().Backup.InspectLicense(ctx, licenseInspectRequestObject)
		if err != nil {
			return "", false, err
		}
		licenseResponseInfoFeatureInfo := licenseInspectResponse.GetLicenseRespInfo().GetFeatureInfo()
		if licenseResponseInfoFeatureInfo[0].Consumed == expectedLicenseConsumedCount {
			return "", false, nil
		}
		return "", true, fmt.Errorf("actual license count:%v, expected license count: %v", licenseInspectResponse.GetLicenseRespInfo().GetFeatureInfo()[0].Consumed, expectedLicenseConsumedCount)
	}
	_, err := task.DoRetryWithTimeout(licenseCountCheck, licenseCountUpdateTimeout, licenseCountUpdateRetryTime)
	if err != nil {
		return err
	}
	return err
}

// DeleteRule deletes backup rule
func DeleteRule(ruleName string, orgId string, ctx context.Context) error {
	ruleUid, err := Inst().Backup.GetRuleUid(orgID, ctx, ruleName)
	if err != nil {
		return err
	}
	deleteRuleReq := &api.RuleDeleteRequest{
		OrgId: orgId,
		Name:  ruleName,
		Uid:   ruleUid,
	}
	_, err = Inst().Backup.DeleteRule(ctx, deleteRuleReq)
	if err != nil {
		return err
	}
	return nil
}

// SafeAppend appends elements to a given slice in a thread-safe manner using a provided mutex
func SafeAppend(mu *sync.Mutex, slice interface{}, elements ...interface{}) interface{} {
	mu.Lock()
	defer mu.Unlock()
	sliceValue := reflect.ValueOf(slice)
	for _, elem := range elements {
		elemValue := reflect.ValueOf(elem)
		sliceValue = reflect.Append(sliceValue, elemValue)
	}
	return sliceValue.Interface()
}

// TaskHandler executes the given task on each input in the taskInputs collection, either sequentially
// * or in parallel, depending on the specified execution mode. It also returns an error when taskInputs is not
// * of type slice or map.
// *
// * Parameters:
// *
// * taskInputs: The collection of inputs to operate on (either a slice or map).
// * task:       The function to execute on each input. If the function takes one argument,
// *
// *	it will be passed the input value. If it takes two arguments, the first
// *	will be the input key or index, and the second will be the input value.
// *
// * executionMode: The mode to use for executing the task, either "Sequential" or "Parallel".
// *
// * # Example
// *
// * The original code:
// *
// *	for _, value := range taskInputs / slice or map / {
// *	    task(value)
// *	}
// *
// * or
// *
// *	for index, value := range taskInputs / slice / {
// *	    task(index, value)
// *	}
// *
// * or
// *
// *	for key, value := range taskInputs / map / {
// *	    task(key, value)
// *	}
// *
// * The original code uses a common pattern for iterating over a slice or map of inputs and calling the 'task'
// * function for each input. To simplify this pattern and allow for concurrent execution of the 'task'
// * function, you can replace the for loops with a call to TaskHandler(taskInputs, task, executionMode), where
// * 'executionMode' is either 'Parallel' or 'Sequential'.
func TaskHandler(taskInputs interface{}, task interface{}, executionMode ExecutionMode) error {
	v := reflect.ValueOf(taskInputs)
	var keys []reflect.Value
	isMap := false
	if v.Kind() == reflect.Map {
		keys = v.MapKeys()
		isMap = true
	} else if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		keys = make([]reflect.Value, v.Len())
		for i := 0; i < v.Len(); i++ {
			keys[i] = v.Index(i)
		}
	} else {
		return fmt.Errorf("instead of %#v, type of taskInputs should be a slice or map", v.Kind().String())
	}
	length := len(keys)
	if length == 0 {
		return nil
	} else if length == 1 {
		executionMode = Sequential
	}
	fnValue := reflect.ValueOf(task)
	numArgs := fnValue.Type().NumIn()
	callTask := func(key, value reflect.Value) {
		in := make([]reflect.Value, numArgs)
		if numArgs == 1 {
			in[0] = value
		} else {
			in[0] = key
			in[1] = value
		}
		fnValue.Call(in)
	}
	if executionMode == Sequential {
		for i := 0; i < length; i++ {
			if isMap {
				callTask(keys[i], v.MapIndex(keys[i]))
			} else {
				callTask(reflect.ValueOf(i), keys[i])
			}
		}
	} else {
		var wg sync.WaitGroup
		for i := 0; i < length; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if isMap {
					callTask(keys[i], v.MapIndex(keys[i]))
				} else {
					callTask(reflect.ValueOf(i), keys[i])
				}
			}(i)
		}
		wg.Wait()
	}
	return nil
}

// FetchNamespacesFromBackup fetches the namespace from backup
func FetchNamespacesFromBackup(ctx context.Context, backupName string, orgID string) ([]string, error) {
	var backedUpNamespaces []string
	backupUid, err := Inst().Backup.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return nil, err
	}
	backupInspectRequest := &api.BackupInspectRequest{
		Name:  backupName,
		Uid:   backupUid,
		OrgId: orgID,
	}
	resp, err := Inst().Backup.InspectBackup(ctx, backupInspectRequest)
	if err != nil {
		return nil, err
	}
	backedUpNamespaces = resp.GetBackup().GetNamespaces()
	return backedUpNamespaces, err
}

// AreSlicesEqual verifies if two slices are equal or not
func AreSlicesEqual(slice1, slice2 interface{}) bool {
	v1 := reflect.ValueOf(slice1)
	v2 := reflect.ValueOf(slice2)
	if v1.Len() != v2.Len() {
		return false
	}
	m := make(map[interface{}]int)
	for i := 0; i < v2.Len(); i++ {
		m[v2.Index(i).Interface()]++
	}
	for i := 0; i < v1.Len(); i++ {
		if m[v1.Index(i).Interface()] == 0 {
			return false
		}
		m[v1.Index(i).Interface()]--
	}
	return true
}

// AreStringSlicesEqual compares two slices of string
func AreStringSlicesEqual(slice1 []string, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	sort.Sort(sort.StringSlice(slice1))
	sort.Sort(sort.StringSlice(slice2))
	for i, v := range slice1 {
		if v != slice2[i] {
			return false
		}
	}
	return true
}

// GetNextScheduleBackupName returns the upcoming schedule backup after it has been initiated
func GetNextScheduleBackupName(scheduleName string, scheduleInterval time.Duration, ctx context.Context) (string, error) {
	var nextScheduleBackupName string
	allScheduleBackupNames, err := Inst().Backup.GetAllScheduleBackupNames(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	currentScheduleBackupCount := len(allScheduleBackupNames)
	nextScheduleBackupOrdinal := currentScheduleBackupCount + 1
	checkOrdinalScheduleBackupCreation := func() (interface{}, bool, error) {
		ordinalScheduleBackupName, err := GetOrdinalScheduleBackupName(ctx, scheduleName, nextScheduleBackupOrdinal, orgID)
		if err != nil {
			return "", true, err
		}
		return ordinalScheduleBackupName, false, nil
	}
	log.InfoD("Waiting for [%d] minutes for the next schedule backup to be triggered", scheduleInterval)
	time.Sleep(scheduleInterval * time.Minute)
	nextScheduleBackup, err := task.DoRetryWithTimeout(checkOrdinalScheduleBackupCreation, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second)
	if err != nil {
		return "", err
	}
	nextScheduleBackupName = nextScheduleBackup.(string)
	return nextScheduleBackupName, nil
}

// GetNextCompletedScheduleBackupName returns the upcoming schedule backup
// after it has been created and checked for success status
func GetNextCompletedScheduleBackupName(ctx context.Context, scheduleName string, scheduleInterval time.Duration) (string, error) {
	nextScheduleBackupName, err := GetNextScheduleBackupName(scheduleName, scheduleInterval, ctx)
	if err != nil {
		return "", err
	}
	log.InfoD("Next schedule backup name [%s]", nextScheduleBackupName)
	err = backupSuccessCheck(nextScheduleBackupName, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return "", err
	}
	return nextScheduleBackupName, nil
}

// GetNextCompletedScheduleBackupNameWithValidation returns the upcoming schedule backup
// after it has been created and checked for success status and validated
func GetNextCompletedScheduleBackupNameWithValidation(ctx context.Context, scheduleName string, scheduledAppContextsToBackup []*scheduler.Context, scheduleInterval time.Duration) (string, error) {
	nextScheduleBackupName, err := GetNextScheduleBackupName(scheduleName, scheduleInterval, ctx)
	if err != nil {
		return "", err
	}
	log.InfoD("Next schedule backup name [%s]", nextScheduleBackupName)
	err = backupSuccessCheckWithValidation(ctx, nextScheduleBackupName, scheduledAppContextsToBackup, orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second)
	if err != nil {
		return "", err
	}
	return nextScheduleBackupName, nil
}

// GetNextPeriodicScheduleBackupName returns next periodic schedule backup name with the given interval
func GetNextPeriodicScheduleBackupName(scheduleName string, scheduleInterval time.Duration, ctx context.Context) (string, error) {
	var nextScheduleBackupName string
	allScheduleBackupNames, err := Inst().Backup.GetAllScheduleBackupNames(ctx, scheduleName, orgID)
	if err != nil {
		return "", err
	}
	currentScheduleBackupCount := len(allScheduleBackupNames)
	nextScheduleBackupOrdinal := currentScheduleBackupCount + 1
	checkOrdinalScheduleBackupCreation := func() (interface{}, bool, error) {
		ordinalScheduleBackupName, err := GetOrdinalScheduleBackupName(ctx, scheduleName, nextScheduleBackupOrdinal, orgID)
		if err != nil {
			return "", true, err
		}
		return ordinalScheduleBackupName, false, nil
	}
	log.InfoD("Waiting for %v minutes for the next schedule backup to be triggered", scheduleInterval)
	time.Sleep(scheduleInterval * time.Minute)
	nextScheduleBackup, err := task.DoRetryWithTimeout(checkOrdinalScheduleBackupCreation, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second)
	if err != nil {
		return "", err
	}
	log.InfoD("Next schedule backup name [%s]", nextScheduleBackup.(string))
	err = backupSuccessCheck(nextScheduleBackup.(string), orgID, maxWaitPeriodForBackupCompletionInMinutes*time.Minute, 30*time.Second, ctx)
	if err != nil {
		return "", err
	}
	nextScheduleBackupName = nextScheduleBackup.(string)
	return nextScheduleBackupName, nil
}

// RemoveElementByValue remove the first occurence of the element from the array.Pass a pointer to the array and the element by value.
func RemoveElementByValue(arr interface{}, value interface{}) error {
	v := reflect.ValueOf(arr)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("removeElementByValue: not a pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("removeElementByValue: not a slice pointer")
	}
	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == value {
			v.Set(reflect.AppendSlice(v.Slice(0, i), v.Slice(i+1, v.Len())))
			break
		}
	}
	return nil
}

// IsFullBackup checks if given backup is full backup or not
func IsFullBackup(backupName string, orgID string, ctx context.Context) error {
	backupUid, err := Inst().Backup.GetBackupUID(ctx, backupName, orgID)
	if err != nil {
		return err
	}
	backupInspectReq := &api.BackupInspectRequest{
		Name:  backupName,
		OrgId: orgID,
		Uid:   backupUid,
	}
	resp, err := Inst().Backup.InspectBackup(ctx, backupInspectReq)
	if err != nil {
		return err
	}
	for _, vol := range resp.GetBackup().GetVolumes() {
		backupId := vol.GetBackupId()
		log.Infof("BackupID of backup [%s]: [%s]", backupName, backupId)
		if strings.HasSuffix(backupId, "-incr") {
			return fmt.Errorf("backup [%s] is an incremental backup", backupName)
		}
	}
	return nil
}

// RemoveLabelFromNodesIfPresent remove the given label from the given node if present
func RemoveLabelFromNodesIfPresent(node node.Node, expectedKey string) error {
	nodeLabels, err := core.Instance().GetLabelsOnNode(node.Name)
	if err != nil {
		return err
	}
	for key := range nodeLabels {
		if key == expectedKey {
			log.InfoD("Removing the applied label with key %s from node %s", expectedKey, node.Name)
			err = Inst().S.RemoveLabelOnNode(node, expectedKey)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

// ValidatePodByLabel validates if the pod with specified label is in a running state
func ValidatePodByLabel(label map[string]string, namespace string, timeout time.Duration, retryInterval time.Duration) error {
	log.Infof("Checking if pods with label %v are running in namespace %s", label, namespace)
	pods, err := core.Instance().GetPods(namespace, label)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err = core.Instance().ValidatePod(&pod, timeout, retryInterval)
		if err != nil {
			return fmt.Errorf("failed to validate pod [%s] with error - %s", pod.GetName(), err.Error())
		}
	}
	return nil
}

// IsMongoDBReady validates if the mongo db pods in Px-Backup namespace are healthy enough for Px-Backup to function
func IsMongoDBReady() error {
	log.Infof("Verify that at least 2 mongodb pods are in Ready state at the end of the testcase")
	pxbNamespace, err := backup.GetPxBackupNamespace()
	if err != nil {
		return err
	}
	mongoDBPodStatus := func() (interface{}, bool, error) {
		statefulSet, err := apps.Instance().GetStatefulSet(mongodbStatefulset, pxbNamespace)
		if err != nil {
			return "", true, err
		}
		// Px-Backup would function with just 2 mongo DB pods in healthy state.
		// Ideally we would expect all 3 pods to be ready but because of intermittent issues, we are limiting to 2
		// TODO: Remove the limit to check for only 2 out of 3 pods once fixed
		// Tracking JIRAs: https://portworx.atlassian.net/browse/PB-3105, https://portworx.atlassian.net/browse/PB-3481
		if statefulSet.Status.ReadyReplicas < 2 {
			return "", true, fmt.Errorf("mongodb pods are not ready yet. expected ready pods - %d, actual ready pods - %d",
				2, statefulSet.Status.ReadyReplicas)
		}
		return "", false, nil
	}
	_, err = DoRetryWithTimeoutWithGinkgoRecover(mongoDBPodStatus, 30*time.Minute, 30*time.Second)
	if err != nil {
		return err
	}
	statefulSet, err := apps.Instance().GetStatefulSet(mongodbStatefulset, pxbNamespace)
	if err != nil {
		return err
	}
	log.Infof("Number of mongodb pods in Ready state are %v", statefulSet.Status.ReadyReplicas)
	return nil
}

// DeleteAppNamespace deletes the given namespace and wait for termination
func DeleteAppNamespace(namespace string) error {
	k8sCore := core.Instance()
	err := k8sCore.DeleteNamespace(namespace)
	if err != nil {
		return err
	}
	namespaceDeleteCheck := func() (interface{}, bool, error) {
		nsObj, err := core.Instance().GetNamespace(namespace)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return "", false, nil
			} else {
				return "", false, err
			}
		}
		if nsObj.Status.Phase == "Terminating" {
			return "", true, fmt.Errorf("namespace - %s is in %s phase ", namespace, nsObj.Status.Phase)
		}
		return "", false, nil
	}
	_, err = task.DoRetryWithTimeout(namespaceDeleteCheck, namespaceDeleteTimeout, jobDeleteRetryTime)
	if err != nil {
		return err
	}
	return nil
}

// RegisterCluster adds the cluster with the given name
func RegisterCluster(clusterName string, cloudCredName string, orgID string, ctx context.Context) error {
	var kubeconfigPath string
	var err error
	kubeConfigs := os.Getenv("KUBECONFIGS")
	if kubeConfigs == "" {
		return fmt.Errorf("unable to get KUBECONFIGS from Environment variable")
	}
	kubeconfigList := strings.Split(kubeConfigs, ",")
	DumpKubeconfigs(kubeconfigList)
	// Register cluster with backup driver
	log.InfoD("Create cluster [%s] in org [%s]", clusterName, orgID)
	if clusterName == SourceClusterName {
		kubeconfigPath, err = GetSourceClusterConfigPath()
	} else if clusterName == destinationClusterName {
		kubeconfigPath, err = GetDestinationClusterConfigPath()
	} else {
		return fmt.Errorf("registering %s cluster not implemented", clusterName)
	}
	if err != nil {
		return err
	}
	log.Infof("Save cluster %s kubeconfig to %s", clusterName, kubeconfigPath)
	clusterStatus := func() (interface{}, bool, error) {
		err = CreateCluster(clusterName, kubeconfigPath, orgID, cloudCredName, "", ctx)
		if err != nil && !strings.Contains(err.Error(), "already exists with status: Online") {
			return "", true, err
		}
		createClusterStatus, err := Inst().Backup.GetClusterStatus(orgID, clusterName, ctx)
		if err != nil {
			return "", true, err
		}
		if createClusterStatus == api.ClusterInfo_StatusInfo_Online {
			return "", false, nil
		}
		return "", true, fmt.Errorf("the %s cluster state is not Online yet", clusterName)
	}
	_, err = task.DoRetryWithTimeout(clusterStatus, clusterCreationTimeout, clusterCreationRetryTime)
	if err != nil {
		return err
	}
	return nil
}

// NamespaceExistsInNamespaceMapping checks if namespace is present in map of namespace mapping
func NamespaceExistsInNamespaceMapping(namespaceMap map[string]string, namespaces []string) bool {
	for _, namespace := range namespaces {
		if _, ok := namespaceMap[namespace]; !ok {
			fmt.Printf("%s is not a present in namespaces %v", namespace, namespaces)
			return false
		}
	}
	return true
}

// RemoveNamespaceLabelForMultipleNamespaces removes labels from multiple namespace
func RemoveNamespaceLabelForMultipleNamespaces(labels map[string]string, namespaces []string) error {
	for _, namespace := range namespaces {
		err := Inst().S.RemoveNamespaceLabel(namespace, labels)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddSourceCluster(ctx context.Context) error {
	err := RegisterCluster(SourceClusterName, "", orgID, ctx)
	if err != nil {
		return err
	}
	return nil
}

func AddDestinationCluster(ctx context.Context) error {
	err := RegisterCluster(destinationClusterName, "", orgID, ctx)
	if err != nil {
		return err
	}
	return nil
}

// GenerateRandomLabelsWithMaxChar creates random label with max characters
func GenerateRandomLabelsWithMaxChar(number int, charLimit int) map[string]string {
	labels := make(map[string]string)
	for i := 0; i < number; i++ {
		key := RandomString(charLimit)
		value := uuid.New()
		labels[key] = value
	}
	return labels
}

// GetCustomBucketName creates a custom bucket and returns name
func GetCustomBucketName(provider string, testName string) string {
	var customBucket string
	customBucket = fmt.Sprintf("%s-%s-%v", provider, testName, time.Now().Unix())
	if provider == drivers.ProviderAws {
		CreateBucket(provider, customBucket)
	}
	return customBucket
}
