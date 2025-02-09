package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/portworx/torpedo/pkg/log"

	. "github.com/onsi/ginkgo"
	"github.com/portworx/sched-ops/k8s/core"
	"github.com/portworx/torpedo/drivers/node"
	"github.com/portworx/torpedo/drivers/scheduler"
	k8s "github.com/portworx/torpedo/drivers/scheduler/k8s"
	. "github.com/portworx/torpedo/tests"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testTriggersConfigMap = "longevity-triggers"
	configMapNS           = "default"
	controlLoopSleepTime  = time.Second * 15
	podDestroyTimeout     = 5 * time.Minute
)

var (
	// Stores mapping between chaos level and its freq. Values are hardcoded
	triggerInterval map[string]map[int]time.Duration
	// Stores which are disruptive triggers. When disruptive triggers are happening in test,
	// other triggers are allowed to happen only after existing triggers are complete.
	disruptiveTriggers map[string]bool

	triggerFunctions     map[string]func(*[]*scheduler.Context, *chan *EventRecord)
	emailTriggerFunction map[string]func()

	// Pure Topology is disabled by default
	pureTopologyEnabled = false

	//Default is allow deploying apps both in storage and storageless nodes
	hyperConvergedTypeEnabled = true

	// Pure Topology Label array
	labels []map[string]string
)

var _ = Describe("{Longevity}", func() {
	contexts := make([]*scheduler.Context, 0)
	var triggerLock sync.Mutex
	var emailTriggerLock sync.Mutex
	var populateDone bool
	triggerEventsChan := make(chan *EventRecord, 100)
	triggerFunctions = map[string]func(*[]*scheduler.Context, *chan *EventRecord){
		DeployApps:       TriggerDeployNewApps,
		RebootNode:       TriggerRebootNodes,
		ValidatePdsApps:  TriggerValidatePdsApps,
		CrashNode:        TriggerCrashNodes,
		RestartVolDriver: TriggerRestartVolDriver,
		CrashVolDriver:   TriggerCrashVolDriver,
		HAIncrease:       TriggerHAIncrease,
		HADecrease:       TriggerHADecrease,
		VolumeClone:      TriggerVolumeClone,
		VolumeResize:     TriggerVolumeResize,
		//EmailReporter:        TriggerEmailReporter,
		AppTaskDown:              TriggerAppTaskDown,
		AppTasksDown:             TriggerAppTasksDown,
		AddDrive:                 TriggerAddDrive,
		CoreChecker:              TriggerCoreChecker,
		CloudSnapShot:            TriggerCloudSnapShot,
		LocalSnapShot:            TriggerLocalSnapShot,
		DeleteLocalSnapShot:      TriggerDeleteLocalSnapShot,
		PoolResizeDisk:           TriggerPoolResizeDisk,
		PoolAddDisk:              TriggerPoolAddDisk,
		UpgradeStork:             TriggerUpgradeStork,
		VolumesDelete:            TriggerVolumeDelete,
		UpgradeVolumeDriver:      TriggerUpgradeVolumeDriver,
		AutoFsTrim:               TriggerAutoFsTrim,
		UpdateVolume:             TriggerVolumeUpdate,
		RestartManyVolDriver:     TriggerRestartManyVolDriver,
		RebootManyNodes:          TriggerRebootManyNodes,
		NodeDecommission:         TriggerNodeDecommission,
		NodeRejoin:               TriggerNodeRejoin,
		CsiSnapShot:              TriggerCsiSnapShot,
		CsiSnapRestore:           TriggerCsiSnapRestore,
		RelaxedReclaim:           TriggerRelaxedReclaim,
		Trashcan:                 TriggerTrashcan,
		KVDBFailover:             TriggerKVDBFailover,
		ValidateDeviceMapper:     TriggerValidateDeviceMapperCleanup,
		MetroDR:                  TriggerMetroDR,
		AsyncDR:                  TriggerAsyncDR,
		AsyncDRMigrationSchedule: TriggerAsyncDRMigrationSchedule,
		ConfluentAsyncDR:         TriggerConfluentAsyncDR,
		AsyncDRVolumeOnly:        TriggerAsyncDRVolumeOnly,
		AutoFsTrimAsyncDR:        TriggerAutoFsTrimAsyncDR,
		IopsBwAsyncDR:            TriggerIopsBwAsyncDR,
		StorkApplicationBackup:   TriggerStorkApplicationBackup,
		StorkAppBkpVolResize:     TriggerStorkAppBkpVolResize,
		StorkAppBkpHaUpdate:      TriggerStorkAppBkpHaUpdate,
		StorkAppBkpPxRestart:     TriggerStorkAppBkpPxRestart,
		StorkAppBkpPoolResize:    TriggerStorkAppBkpPoolResize,
		RestartKvdbVolDriver:     TriggerRestartKvdbVolDriver,
		HAIncreaseAndReboot:      TriggerHAIncreaseAndReboot,
		AddDiskAndReboot:         TriggerPoolAddDiskAndReboot,
		ResizeDiskAndReboot:      TriggerPoolResizeDiskAndReboot,
		AutopilotRebalance:       TriggerAutopilotPoolRebalance,
		VolumeCreatePxRestart:    TriggerVolumeCreatePXRestart,
		DeleteOldNamespaces:      TriggerDeleteOldNamespaces,
		MetroDRMigrationSchedule: TriggerMetroDRMigrationSchedule,
	}
	//Creating a distinct trigger to make sure email triggers at regular intervals
	emailTriggerFunction = map[string]func(){
		EmailReporter: TriggerEmailReporter,
	}

	BeforeEach(func() {
		if !populateDone {
			tags := map[string]string{
				"longevity": "true",
			}
			StartTorpedoTest("PX-Longevity", "Validate PX longevity workflow", tags, 0)

			populateIntervals()
			populateDisruptiveTriggers()
			populateDone = true
		}
	})

	It("has to schedule app and introduce test triggers", func() {
		log.InfoD("schedule apps and start test triggers")
		watchLog := fmt.Sprintf("Start watch on K8S configMap [%s/%s]",
			configMapNS, testTriggersConfigMap)

		Step(watchLog, func() {
			log.InfoD(watchLog)
			err := watchConfigMap()
			if err != nil {
				log.Fatalf(fmt.Sprintf("%v", err))
			}
		})

		if pureTopologyEnabled {
			var err error
			labels, err = SetTopologyLabelsOnNodes()
			if err != nil {
				log.Fatalf(fmt.Sprintf("%v", err))
			}
			Inst().TopologyLabels = labels
		}

		Inst().IsHyperConverged = hyperConvergedTypeEnabled

		TriggerDeployNewApps(&contexts, &triggerEventsChan)

		var wg sync.WaitGroup
		Step("Register test triggers", func() {
			for triggerType, triggerFunc := range triggerFunctions {
				log.InfoD("Registering trigger: [%v]", triggerType)
				go testTrigger(&wg, &contexts, triggerType, triggerFunc, &triggerLock, &triggerEventsChan)
				wg.Add(1)
			}
		})
		log.InfoD("Finished registering test triggers")
		if Inst().MinRunTimeMins != 0 {
			log.InfoD("Longevity Tests  timeout set to %d  minutes", Inst().MinRunTimeMins)
		}

		Step("Register email trigger", func() {
			for triggerType, triggerFunc := range emailTriggerFunction {
				log.InfoD("Registering email trigger: [%v]", triggerType)
				go emailEventTrigger(&wg, triggerType, triggerFunc, &emailTriggerLock)
				wg.Add(1)
			}
		})
		log.InfoD("Finished registering email trigger")

		CollectEventRecords(&triggerEventsChan)
		wg.Wait()
		close(triggerEventsChan)
		Step("teardown all apps", func() {
			for _, ctx := range contexts {
				TearDownContext(ctx, nil)
			}
		})
	})
	JustAfterEach(func() {
		defer EndTorpedoTest()
		AfterEachTest(contexts)
	})
})

func testTrigger(wg *sync.WaitGroup,
	contexts *[]*scheduler.Context,
	triggerType string,
	triggerFunc func(*[]*scheduler.Context, *chan *EventRecord),
	triggerLoc *sync.Mutex,
	triggerEventsChan *chan *EventRecord) {
	defer wg.Done()

	minRunTime := Inst().MinRunTimeMins
	timeout := (minRunTime) * 60

	start := time.Now().Local()
	lastInvocationTime := start

	for {
		// if timeout is 0, run indefinitely
		if timeout != 0 && int(time.Since(start).Seconds()) > timeout {
			log.InfoD("Longevity Tests timed out with timeout %d  minutes", minRunTime)
			break
		}

		// Get next interval of when trigger should happen
		// This interval can dynamically change by editing configMap
		waitTime, isTriggerEnabled := isTriggerEnabled(triggerType)

		if isTriggerEnabled && time.Since(lastInvocationTime) > time.Duration(waitTime) {
			// If trigger is not disabled and its right time to trigger,

			log.Infof("Waiting for lock for trigger [%s]\n", triggerType)
			triggerLoc.Lock()
			log.Infof("Successfully taken lock for trigger [%s]\n", triggerType)
			/* PTX-2667: check no other disruptive trigger is happening at same time
			if isDisruptiveTrigger(triggerType) {
			   // At a give point in time, only single disruptive trigger is allowed to run.
			   // No other disruptive or non-disruptive trigger can run at this time.
			   triggerLoc.Lock()
			} else {
			   // If trigger is non-disruptive then just check if no other disruptive trigger is running or not
			   // and release the lock immidiately so that other non-disruptive triggers can happen.
				triggerLoc.Lock()
				log.Infof("===No other disruptive event happening. Able to take lock for [%s]\n", triggerType)
				triggerLoc.Unlock()
				log.Infof("===Releasing lock for non-disruptive event [%s]\n", triggerType)
			}*/

			triggerFunc(contexts, triggerEventsChan)
			log.Infof("Trigger Function completed for [%s]\n", triggerType)

			//if isDisruptiveTrigger(triggerType) {
			triggerLoc.Unlock()
			log.Infof("Successfully released lock for trigger [%s]\n", triggerType)
			//}

			lastInvocationTime = time.Now().Local()

		}
		time.Sleep(controlLoopSleepTime)
	}
	os.Exit(0)
}

func emailEventTrigger(wg *sync.WaitGroup,
	triggerType string,
	triggerFunc func(),
	emailTriggerLock *sync.Mutex) {
	defer wg.Done()

	minRunTime := Inst().MinRunTimeMins
	timeout := (minRunTime) * 60

	start := time.Now().Local()
	lastInvocationTime := start

	for {
		// if timeout is 0, run indefinitely
		if timeout != 0 && int(time.Since(start).Seconds()) > timeout {
			break
		}

		// Get next interval of when trigger should happen
		// This interval can dynamically change by editing configMap
		waitTime, isTriggerEnabled := isTriggerEnabled(triggerType)

		if isTriggerEnabled && time.Since(lastInvocationTime) > time.Duration(waitTime) {
			// If trigger is not disabled and its right time to trigger,

			log.InfoD("Waiting for lock for trigger [%s]\n", triggerType)
			emailTriggerLock.Lock()
			log.InfoD("Successfully taken lock for trigger [%s]\n", triggerType)

			triggerFunc()
			log.InfoD("Trigger Function completed for [%s]\n", triggerType)

			emailTriggerLock.Unlock()
			log.InfoD("Successfully released lock for trigger [%s]\n", triggerType)

			lastInvocationTime = time.Now().Local()

		}
		time.Sleep(controlLoopSleepTime)
	}
}

func watchConfigMap() error {
	ChaosMap = map[string]int{}
	cm, err := core.Instance().GetConfigMap(testTriggersConfigMap, configMapNS)
	if err != nil {
		return fmt.Errorf("Error reading config map: %v", err)
	}
	err = populateDataFromConfigMap(&cm.Data)
	if err != nil {
		return err
	}

	// Apply watch if configMap exists
	fn := func(object runtime.Object) error {
		cm, ok := object.(*v1.ConfigMap)
		if !ok {
			err := fmt.Errorf("invalid object type on configmap watch: %v", object)
			return err
		}
		if len(cm.Data) > 0 {
			err = populateDataFromConfigMap(&cm.Data)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err = core.Instance().WatchConfigMap(cm, fn)
	if err != nil {
		return fmt.Errorf("Failed to watch on config map: %s due to: %v", testTriggersConfigMap, err)
	}
	return nil
}

func populateDisruptiveTriggers() {
	disruptiveTriggers = map[string]bool{
		HAIncrease:                      false,
		HADecrease:                      false,
		RestartVolDriver:                false,
		CrashVolDriver:                  false,
		RebootNode:                      true,
		CrashNode:                       true,
		EmailReporter:                   false,
		AppTaskDown:                     false,
		DeployApps:                      false,
		BackupAllApps:                   false,
		BackupScheduleAll:               false,
		BackupScheduleScale:             true,
		BackupSpecificResource:          false,
		BackupSpecificResourceOnCluster: false,
		TestInspectBackup:               false,
		TestInspectRestore:              false,
		TestDeleteBackup:                false,
		RestoreNamespace:                false,
		BackupUsingLabelOnCluster:       false,
		BackupRestartPX:                 false,
		BackupRestartNode:               false,
		BackupDeleteBackupPod:           false,
		BackupScaleMongo:                false,
		AppTasksDown:                    false,
		RestartManyVolDriver:            true,
		RebootManyNodes:                 true,
		RestartKvdbVolDriver:            true,
		NodeDecommission:                true,
		CsiSnapShot:                     false,
		CsiSnapRestore:                  false,
		KVDBFailover:                    true,
		HAIncreaseAndReboot:             true,
		AddDiskAndReboot:                true,
		ResizeDiskAndReboot:             true,
		VolumeCreatePxRestart:           true,
	}
}

func isDisruptiveTrigger(triggerType string) bool {
	return disruptiveTriggers[triggerType]
}

func populateDataFromConfigMap(configData *map[string]string) error {
	setEmailRecipients(configData)
	setEmailHost(configData)
	setEmailSubject(configData)
	setPureTopology(configData)
	setHyperConvergedType(configData)
	setMigrationInterval(configData)
	setMigrationsCount(configData)
	setCreatedBeforeTimeForNsDeletion(configData)
	err := setSendGridEmailAPIKey(configData)
	if err != nil {
		return err
	}

	err = populateTriggers(configData)
	if err != nil {
		return err
	}
	return nil
}

func setEmailRecipients(configData *map[string]string) {
	// Get email recipients from configMap
	if emailRecipients, ok := (*configData)[EmailRecipientsConfigMapField]; !ok {
		log.Warnf("No [%s] field found in [%s] config-map in [%s] namespace."+
			"Defaulting email recipients to [%s].\n",
			EmailRecipientsConfigMapField, testTriggersConfigMap, configMapNS, DefaultEmailRecipient)
		EmailRecipients = []string{DefaultEmailRecipient}
	} else {
		EmailRecipients = strings.Split(emailRecipients, ",")
		delete(*configData, EmailRecipientsConfigMapField)
	}
}

func setEmailHost(configData *map[string]string) error {
	if emailhost, ok := (*configData)[EmailHostServerField]; ok {
		EmailServer = emailhost
		delete(*configData, EmailHostServerField)
		return nil
	}
	return fmt.Errorf("Failed to find [%s] field in config-map [%s] in namespace [%s]",
		EmailHostServerField, testTriggersConfigMap, configMapNS)
}

func setEmailSubject(configData *map[string]string) {
	if emailsubject, ok := (*configData)[EmailSubjectField]; ok {
		EmailSubject = emailsubject
		delete(*configData, EmailSubjectField)
	} else {
		EmailSubject = "Torpedo Longevity Report"
	}
}

func setMigrationInterval(configData *map[string]string) {
	var err error
	if migInt, ok := (*configData)[MigrationIntervalField]; ok {
		MigrationInterval, err = strconv.Atoi(migInt)
		if err != nil {
			log.Errorf("Cannot set Migration interval value, getting error: %v", err)
		}
	} else {
		MigrationInterval = 3
	}
}

func setMigrationsCount(configData *map[string]string) {
	var err error
	if migCount, ok := (*configData)[MigrationsCountField]; ok {
		MigrationsCount, err = strconv.Atoi(migCount)
		if err != nil {
			log.Errorf("Cannot set Migration Count value, getting error: %v", err)
		}
	} else {
		MigrationsCount = 5
	}
}

func setCreatedBeforeTimeForNsDeletion(configData *map[string]string) {
	var err error
	if createdbeforetimeforns, ok := (*configData)[CreatedBeforeTimeForNsField]; ok {
		CreatedBeforeTimeforNS, err = strconv.Atoi(createdbeforetimeforns)
		if err != nil {
			log.Errorf("Cannot set createdbeforetimeforns value, getting error: %v", err)
		}
	} else {
		CreatedBeforeTimeforNS = 0
	}
}

// setPureTopology read the config map and set the pureTopologyEnabled field
func setPureTopology(configData *map[string]string) {
	// Set Pure Topology Enabled value from configMap
	var err error
	if pureTopology, ok := (*configData)[PureTopologyField]; !ok {
		log.Warnf("No [%s] field found in [%s] config-map in [%s] namespace.\n",
			PureTopologyField, testTriggersConfigMap, configMapNS)
	} else {
		pureTopologyEnabled, err = strconv.ParseBool(pureTopology)
		if err != nil {
			log.Errorf("Failed to parse [%s] field in config-map in [%s] namespace.Error:[%v]\n",
				PureTopologyField, configMapNS, err)
		}
		delete(*configData, PureTopologyField)
	}
}

func setHyperConvergedType(configData *map[string]string) {
	var err error
	if hyperConvergedType, ok := (*configData)[HyperConvergedTypeField]; !ok {
		log.Warnf("No [%s] field found in [%s] config-map in [%s] namespace.\n",
			HyperConvergedTypeField, testTriggersConfigMap, configMapNS)
	} else {
		hyperConvergedTypeEnabled, err = strconv.ParseBool(hyperConvergedType)
		if err != nil {
			log.Errorf("Failed to parse [%s] field in config-map in [%s] namespace.Error:[%v]\n",
				HyperConvergedTypeField, configMapNS, err)
		}
		delete(*configData, HyperConvergedTypeField)
	}
}

func setSendGridEmailAPIKey(configData *map[string]string) error {
	if apiKey, ok := (*configData)[SendGridEmailAPIKeyField]; ok {
		SendGridEmailAPIKey = apiKey
		delete(*configData, SendGridEmailAPIKeyField)
		return nil
	}
	return fmt.Errorf("Failed to find [%s] field in config-map [%s] in namespace [%s]",
		SendGridEmailAPIKeyField, testTriggersConfigMap, configMapNS)
}

func populateTriggers(triggers *map[string]string) error {
	for triggerType, chaosLevel := range *triggers {
		chaosLevelInt, err := strconv.Atoi(chaosLevel)
		if err != nil {
			return fmt.Errorf("Failed to get chaos levels from configMap [%s] in [%s] namespace. Error:[%v]",
				testTriggersConfigMap, configMapNS, err)
		}
		ChaosMap[triggerType] = chaosLevelInt
		if triggerType == BackupScheduleAll || triggerType == BackupScheduleScale {
			SetScheduledBackupInterval(triggerInterval[triggerType][chaosLevelInt], triggerType)
		}
	}

	RunningTriggers = map[string]time.Duration{}
	for triggerType := range triggerFunctions {
		chaosLevel, ok := ChaosMap[triggerType]
		if !ok {
			chaosLevel = Inst().ChaosLevel
		}
		if chaosLevel != 0 {
			RunningTriggers[triggerType] = triggerInterval[triggerType][chaosLevel]
		}

	}
	return nil
}

// SetTopologyLabelsOnNodes distribute labels on node
func SetTopologyLabelsOnNodes() ([]map[string]string, error) {
	// Slice of FA labels
	topologyLabels := make([]map[string]string, 0)
	nodeUpTimeout := 5 * time.Minute

	log.Info("Add Topology Labels on node")
	var secret PureSecret
	volumeDriverNamespace, err := Inst().V.GetVolumeDriverNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get volume driver namespace. Error [%v]", err)
	}
	pureSecretString, err := Inst().S.GetSecretData(
		volumeDriverNamespace, PureSecretName, PureSecretDataField,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to read pure secret [%s]. Error [%v]",
			PureSecretName, err)
	}

	pureSecretJSON := []byte(pureSecretString)

	if err = json.Unmarshal(pureSecretJSON, &secret); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal pure secret data [%s]. Error:[%v]",
			pureSecretJSON, err)
	}

	// Appending the labels to a list
	for _, fa := range secret.FlashArrays {
		topologyLabels = append(topologyLabels, fa.Labels)
	}

	topologyGroups := len(topologyLabels)

	// Adding the labels on node.
	for nodeIdx, n := range node.GetStorageDriverNodes() {
		labelIdx := int32(nodeIdx % topologyGroups)
		for key, value := range topologyLabels[labelIdx] {
			if err = Inst().S.AddLabelOnNode(n, key, value); err != nil {
				return nil, fmt.Errorf("Failed to add label key [%s] and value [%s] in node [%s]. Error:[%v]",
					key, value, n.Name, err)
			}
			switch key {
			case k8s.TopologyZoneK8sNodeLabel:
				log.InfoD("Setting node: [%s] Topology Zone to: [%s]", n.Name, value)
				n.TopologyZone = value
			case k8s.TopologyRegionK8sNodeLabel:
				log.InfoD("Setting node: [%s] Topology Region to: [%s]", n.Name, value)
				n.TopologyRegion = value
			}
		}
		// Updating the node with topology info in node registry
		node.UpdateNode(n)
	}

	// Bouncing Back the PX pods on all nodes to restart Csi Registrar Container
	log.Info("Bouncing back the PX pods after setting the Topology Labels on Nodes")

	if err := deletePXPods(volumeDriverNamespace); err != nil {
		return nil, fmt.Errorf("Failed to delete PX pods. Error:[%v]", err)
	}

	// Wait for PX pods to be up
	log.Info("Waiting for Volume Driver to be up and running")
	for _, n := range node.GetStorageDriverNodes() {
		if err := Inst().V.WaitForPxPodsToBeUp(n); err != nil {
			return nil, fmt.Errorf("PX pod not coming up in a node [%s]. Error:[%v]", n.Name, err)
		}
		if err := Inst().V.WaitDriverUpOnNode(n, nodeUpTimeout); err != nil {
			return nil, fmt.Errorf("Volume Driver not coming up in a node [%s]. Error:[%v]", n.Name, err)
		}
	}

	return topologyLabels, nil
}

// deletePXPods delete px pods
func deletePXPods(nameSpace string) error {
	pxLabel := make(map[string]string)
	pxLabel["name"] = "portworx"
	if err := core.Instance().DeletePodsByLabels(nameSpace, pxLabel, podDestroyTimeout); err != nil {
		return err
	}
	return nil
}

func populateIntervals() {
	triggerInterval = map[string]map[int]time.Duration{}
	triggerInterval[ValidatePdsApps] = map[int]time.Duration{}
	triggerInterval[RebootNode] = map[int]time.Duration{}
	triggerInterval[CrashNode] = map[int]time.Duration{}
	triggerInterval[CrashVolDriver] = map[int]time.Duration{}
	triggerInterval[RestartVolDriver] = map[int]time.Duration{}
	triggerInterval[RestartKvdbVolDriver] = map[int]time.Duration{}
	triggerInterval[HAIncrease] = map[int]time.Duration{}
	triggerInterval[HADecrease] = map[int]time.Duration{}
	triggerInterval[EmailReporter] = map[int]time.Duration{}
	triggerInterval[AppTaskDown] = map[int]time.Duration{}
	triggerInterval[DeployApps] = map[int]time.Duration{}
	triggerInterval[CoreChecker] = map[int]time.Duration{}
	triggerInterval[VolumeClone] = map[int]time.Duration{}
	triggerInterval[VolumeResize] = make(map[int]time.Duration)
	triggerInterval[PoolResizeDisk] = make(map[int]time.Duration)
	triggerInterval[PoolAddDisk] = make(map[int]time.Duration)
	triggerInterval[BackupAllApps] = map[int]time.Duration{}
	triggerInterval[BackupScheduleAll] = map[int]time.Duration{}
	triggerInterval[BackupScheduleScale] = map[int]time.Duration{}
	triggerInterval[BackupSpecificResource] = map[int]time.Duration{}
	triggerInterval[BackupSpecificResourceOnCluster] = map[int]time.Duration{}
	triggerInterval[TestInspectRestore] = map[int]time.Duration{}
	triggerInterval[TestInspectBackup] = map[int]time.Duration{}
	triggerInterval[TestDeleteBackup] = map[int]time.Duration{}
	triggerInterval[RestoreNamespace] = map[int]time.Duration{}
	triggerInterval[BackupUsingLabelOnCluster] = map[int]time.Duration{}
	triggerInterval[BackupRestartPX] = map[int]time.Duration{}
	triggerInterval[BackupRestartNode] = map[int]time.Duration{}
	triggerInterval[BackupDeleteBackupPod] = map[int]time.Duration{}
	triggerInterval[BackupScaleMongo] = map[int]time.Duration{}
	triggerInterval[CloudSnapShot] = make(map[int]time.Duration)
	triggerInterval[UpgradeStork] = make(map[int]time.Duration)
	triggerInterval[VolumesDelete] = make(map[int]time.Duration)
	triggerInterval[LocalSnapShot] = make(map[int]time.Duration)
	triggerInterval[DeleteLocalSnapShot] = make(map[int]time.Duration)
	triggerInterval[UpgradeVolumeDriver] = make(map[int]time.Duration)
	triggerInterval[AppTasksDown] = make(map[int]time.Duration)
	triggerInterval[AutoFsTrim] = make(map[int]time.Duration)
	triggerInterval[UpdateVolume] = make(map[int]time.Duration)
	triggerInterval[RestartManyVolDriver] = make(map[int]time.Duration)
	triggerInterval[RebootManyNodes] = make(map[int]time.Duration)
	triggerInterval[NodeDecommission] = make(map[int]time.Duration)
	triggerInterval[NodeRejoin] = make(map[int]time.Duration)
	triggerInterval[CsiSnapShot] = make(map[int]time.Duration)
	triggerInterval[CsiSnapRestore] = make(map[int]time.Duration)
	triggerInterval[RelaxedReclaim] = make(map[int]time.Duration)
	triggerInterval[Trashcan] = make(map[int]time.Duration)
	triggerInterval[KVDBFailover] = make(map[int]time.Duration)
	triggerInterval[ValidateDeviceMapper] = make(map[int]time.Duration)
	triggerInterval[MetroDR] = make(map[int]time.Duration)
	triggerInterval[MetroDRMigrationSchedule] = make(map[int]time.Duration)
	triggerInterval[AsyncDR] = make(map[int]time.Duration)
	triggerInterval[AsyncDRMigrationSchedule] = make(map[int]time.Duration)
	triggerInterval[DeleteOldNamespaces] = make(map[int]time.Duration)
	triggerInterval[AutoFsTrimAsyncDR] = make(map[int]time.Duration)
	triggerInterval[IopsBwAsyncDR] = make(map[int]time.Duration)
	triggerInterval[ConfluentAsyncDR] = make(map[int]time.Duration)
	triggerInterval[AsyncDRVolumeOnly] = make(map[int]time.Duration)
	triggerInterval[StorkApplicationBackup] = make(map[int]time.Duration)
	triggerInterval[StorkAppBkpVolResize] = make(map[int]time.Duration)
	triggerInterval[StorkAppBkpHaUpdate] = make(map[int]time.Duration)
	triggerInterval[StorkAppBkpPxRestart] = make(map[int]time.Duration)
	triggerInterval[StorkAppBkpPoolResize] = make(map[int]time.Duration)
	triggerInterval[HAIncreaseAndReboot] = make(map[int]time.Duration)
	triggerInterval[AddDrive] = make(map[int]time.Duration)
	triggerInterval[AddDiskAndReboot] = make(map[int]time.Duration)
	triggerInterval[ResizeDiskAndReboot] = make(map[int]time.Duration)
	triggerInterval[AutopilotRebalance] = make(map[int]time.Duration)
	triggerInterval[VolumeCreatePxRestart] = make(map[int]time.Duration)

	baseInterval := 10 * time.Minute
	triggerInterval[BackupScaleMongo][10] = 1 * baseInterval
	triggerInterval[BackupScaleMongo][9] = 2 * baseInterval
	triggerInterval[BackupScaleMongo][8] = 3 * baseInterval
	triggerInterval[BackupScaleMongo][7] = 4 * baseInterval
	triggerInterval[BackupScaleMongo][6] = 5 * baseInterval
	triggerInterval[BackupScaleMongo][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[BackupAllApps][10] = 1 * baseInterval
	triggerInterval[BackupAllApps][9] = 2 * baseInterval
	triggerInterval[BackupAllApps][8] = 3 * baseInterval
	triggerInterval[BackupAllApps][7] = 4 * baseInterval
	triggerInterval[BackupAllApps][6] = 5 * baseInterval
	triggerInterval[BackupAllApps][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[BackupScheduleAll][10] = 1 * baseInterval
	triggerInterval[BackupScheduleAll][9] = 2 * baseInterval
	triggerInterval[BackupScheduleAll][8] = 3 * baseInterval
	triggerInterval[BackupScheduleAll][7] = 4 * baseInterval
	triggerInterval[BackupScheduleAll][6] = 5 * baseInterval
	triggerInterval[BackupScheduleAll][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[BackupScheduleScale][10] = 1 * baseInterval
	triggerInterval[BackupScheduleScale][9] = 2 * baseInterval
	triggerInterval[BackupScheduleScale][8] = 3 * baseInterval
	triggerInterval[BackupScheduleScale][7] = 4 * baseInterval
	triggerInterval[BackupScheduleScale][6] = 5 * baseInterval
	triggerInterval[BackupScheduleScale][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[TestInspectRestore][10] = 1 * baseInterval
	triggerInterval[TestInspectRestore][9] = 2 * baseInterval
	triggerInterval[TestInspectRestore][8] = 3 * baseInterval
	triggerInterval[TestInspectRestore][7] = 4 * baseInterval
	triggerInterval[TestInspectRestore][6] = 5 * baseInterval
	triggerInterval[TestInspectRestore][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[TestInspectBackup][10] = 1 * baseInterval
	triggerInterval[TestInspectBackup][9] = 2 * baseInterval
	triggerInterval[TestInspectBackup][8] = 3 * baseInterval
	triggerInterval[TestInspectBackup][7] = 4 * baseInterval
	triggerInterval[TestInspectBackup][6] = 5 * baseInterval
	triggerInterval[TestInspectBackup][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[TestDeleteBackup][10] = 1 * baseInterval
	triggerInterval[TestDeleteBackup][9] = 2 * baseInterval
	triggerInterval[TestDeleteBackup][8] = 3 * baseInterval
	triggerInterval[TestDeleteBackup][7] = 4 * baseInterval
	triggerInterval[TestDeleteBackup][6] = 5 * baseInterval
	triggerInterval[TestDeleteBackup][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[RestoreNamespace][10] = 1 * baseInterval
	triggerInterval[RestoreNamespace][9] = 2 * baseInterval
	triggerInterval[RestoreNamespace][8] = 3 * baseInterval
	triggerInterval[RestoreNamespace][7] = 4 * baseInterval
	triggerInterval[RestoreNamespace][6] = 5 * baseInterval
	triggerInterval[RestoreNamespace][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[TestInspectRestore][10] = 1 * baseInterval
	triggerInterval[TestInspectRestore][9] = 2 * baseInterval
	triggerInterval[TestInspectRestore][8] = 3 * baseInterval
	triggerInterval[TestInspectRestore][7] = 4 * baseInterval
	triggerInterval[TestInspectRestore][6] = 5 * baseInterval
	triggerInterval[TestInspectRestore][5] = 6 * baseInterval

	triggerInterval[TestInspectBackup][10] = 1 * baseInterval
	triggerInterval[TestInspectBackup][9] = 2 * baseInterval
	triggerInterval[TestInspectBackup][8] = 3 * baseInterval
	triggerInterval[TestInspectBackup][7] = 4 * baseInterval
	triggerInterval[TestInspectBackup][6] = 5 * baseInterval
	triggerInterval[TestInspectBackup][5] = 6 * baseInterval

	triggerInterval[TestDeleteBackup][10] = 1 * baseInterval
	triggerInterval[TestDeleteBackup][9] = 2 * baseInterval
	triggerInterval[TestDeleteBackup][8] = 3 * baseInterval
	triggerInterval[TestDeleteBackup][7] = 4 * baseInterval
	triggerInterval[TestDeleteBackup][6] = 5 * baseInterval
	triggerInterval[TestDeleteBackup][5] = 6 * baseInterval

	triggerInterval[RestoreNamespace][10] = 1 * baseInterval
	triggerInterval[RestoreNamespace][9] = 2 * baseInterval
	triggerInterval[RestoreNamespace][8] = 3 * baseInterval
	triggerInterval[RestoreNamespace][7] = 4 * baseInterval
	triggerInterval[RestoreNamespace][6] = 5 * baseInterval
	triggerInterval[RestoreNamespace][5] = 6 * baseInterval

	triggerInterval[BackupSpecificResource][10] = 1 * baseInterval
	triggerInterval[BackupSpecificResource][9] = 2 * baseInterval
	triggerInterval[BackupSpecificResource][8] = 3 * baseInterval
	triggerInterval[BackupSpecificResource][7] = 4 * baseInterval
	triggerInterval[BackupSpecificResource][6] = 5 * baseInterval
	triggerInterval[BackupSpecificResource][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[BackupSpecificResourceOnCluster][10] = 1 * baseInterval
	triggerInterval[BackupSpecificResourceOnCluster][9] = 2 * baseInterval
	triggerInterval[BackupSpecificResourceOnCluster][8] = 3 * baseInterval
	triggerInterval[BackupSpecificResourceOnCluster][7] = 4 * baseInterval
	triggerInterval[BackupSpecificResourceOnCluster][6] = 5 * baseInterval
	triggerInterval[BackupSpecificResourceOnCluster][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[BackupUsingLabelOnCluster][10] = 1 * baseInterval
	triggerInterval[BackupUsingLabelOnCluster][9] = 2 * baseInterval
	triggerInterval[BackupUsingLabelOnCluster][8] = 3 * baseInterval
	triggerInterval[BackupUsingLabelOnCluster][7] = 4 * baseInterval
	triggerInterval[BackupUsingLabelOnCluster][6] = 5 * baseInterval
	triggerInterval[BackupUsingLabelOnCluster][5] = 6 * baseInterval

	triggerInterval[BackupRestartPX][10] = 1 * baseInterval
	triggerInterval[BackupRestartPX][9] = 2 * baseInterval
	triggerInterval[BackupRestartPX][8] = 3 * baseInterval
	triggerInterval[BackupRestartPX][7] = 4 * baseInterval
	triggerInterval[BackupRestartPX][6] = 5 * baseInterval
	triggerInterval[BackupRestartPX][5] = 6 * baseInterval

	triggerInterval[BackupRestartNode][10] = 1 * baseInterval
	triggerInterval[BackupRestartNode][9] = 2 * baseInterval
	triggerInterval[BackupRestartNode][8] = 3 * baseInterval
	triggerInterval[BackupRestartNode][7] = 4 * baseInterval
	triggerInterval[BackupRestartNode][6] = 5 * baseInterval
	triggerInterval[BackupRestartNode][5] = 6 * baseInterval

	triggerInterval[MetroDR][10] = 1 * baseInterval
	triggerInterval[MetroDR][9] = 3 * baseInterval
	triggerInterval[MetroDR][8] = 6 * baseInterval
	triggerInterval[MetroDR][7] = 9 * baseInterval
	triggerInterval[MetroDR][6] = 12 * baseInterval
	triggerInterval[MetroDR][5] = 15 * baseInterval
	triggerInterval[MetroDR][4] = 18 * baseInterval
	triggerInterval[MetroDR][3] = 21 * baseInterval
	triggerInterval[MetroDR][2] = 24 * baseInterval
	triggerInterval[MetroDR][1] = 27 * baseInterval

	triggerInterval[MetroDRMigrationSchedule][10] = 1 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][9] = 3 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][8] = 6 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][7] = 9 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][6] = 12 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][5] = 15 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][4] = 18 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][3] = 21 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][2] = 24 * baseInterval
	triggerInterval[MetroDRMigrationSchedule][1] = 27 * baseInterval

	triggerInterval[AsyncDR][10] = 1 * baseInterval
	triggerInterval[AsyncDR][9] = 3 * baseInterval
	triggerInterval[AsyncDR][8] = 6 * baseInterval
	triggerInterval[AsyncDR][7] = 9 * baseInterval
	triggerInterval[AsyncDR][6] = 12 * baseInterval
	triggerInterval[AsyncDR][5] = 15 * baseInterval
	triggerInterval[AsyncDR][4] = 18 * baseInterval
	triggerInterval[AsyncDR][3] = 21 * baseInterval
	triggerInterval[AsyncDR][2] = 24 * baseInterval
	triggerInterval[AsyncDR][1] = 27 * baseInterval

	triggerInterval[AsyncDRMigrationSchedule][10] = 1 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][9] = 3 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][8] = 6 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][7] = 9 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][6] = 12 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][5] = 15 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][4] = 18 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][3] = 21 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][2] = 24 * baseInterval
	triggerInterval[AsyncDRMigrationSchedule][1] = 27 * baseInterval

	triggerInterval[AutoFsTrimAsyncDR][10] = 1 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][9] = 3 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][8] = 6 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][7] = 9 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][6] = 12 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][5] = 15 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][4] = 18 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][3] = 21 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][2] = 24 * baseInterval
	triggerInterval[AutoFsTrimAsyncDR][1] = 27 * baseInterval

	triggerInterval[IopsBwAsyncDR][10] = 1 * baseInterval
	triggerInterval[IopsBwAsyncDR][9] = 3 * baseInterval
	triggerInterval[IopsBwAsyncDR][8] = 6 * baseInterval
	triggerInterval[IopsBwAsyncDR][7] = 9 * baseInterval
	triggerInterval[IopsBwAsyncDR][6] = 12 * baseInterval
	triggerInterval[IopsBwAsyncDR][5] = 15 * baseInterval
	triggerInterval[IopsBwAsyncDR][4] = 18 * baseInterval
	triggerInterval[IopsBwAsyncDR][3] = 21 * baseInterval
	triggerInterval[IopsBwAsyncDR][2] = 24 * baseInterval
	triggerInterval[IopsBwAsyncDR][1] = 27 * baseInterval

	triggerInterval[ConfluentAsyncDR][10] = 1 * baseInterval
	triggerInterval[ConfluentAsyncDR][9] = 3 * baseInterval
	triggerInterval[ConfluentAsyncDR][8] = 6 * baseInterval
	triggerInterval[ConfluentAsyncDR][7] = 9 * baseInterval
	triggerInterval[ConfluentAsyncDR][6] = 12 * baseInterval
	triggerInterval[ConfluentAsyncDR][5] = 15 * baseInterval
	triggerInterval[ConfluentAsyncDR][4] = 18 * baseInterval
	triggerInterval[ConfluentAsyncDR][3] = 21 * baseInterval
	triggerInterval[ConfluentAsyncDR][2] = 24 * baseInterval
	triggerInterval[ConfluentAsyncDR][1] = 27 * baseInterval

	triggerInterval[AsyncDRVolumeOnly][10] = 1 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][9] = 3 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][8] = 6 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][7] = 9 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][6] = 12 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][5] = 15 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][4] = 18 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][3] = 21 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][2] = 24 * baseInterval
	triggerInterval[AsyncDRVolumeOnly][1] = 27 * baseInterval

	triggerInterval[StorkApplicationBackup][10] = 1 * baseInterval
	triggerInterval[StorkApplicationBackup][9] = 3 * baseInterval
	triggerInterval[StorkApplicationBackup][8] = 6 * baseInterval
	triggerInterval[StorkApplicationBackup][7] = 9 * baseInterval
	triggerInterval[StorkApplicationBackup][6] = 12 * baseInterval
	triggerInterval[StorkApplicationBackup][5] = 15 * baseInterval
	triggerInterval[StorkApplicationBackup][4] = 18 * baseInterval
	triggerInterval[StorkApplicationBackup][3] = 21 * baseInterval
	triggerInterval[StorkApplicationBackup][2] = 24 * baseInterval
	triggerInterval[StorkApplicationBackup][1] = 27 * baseInterval

	triggerInterval[StorkAppBkpVolResize][10] = 1 * baseInterval
	triggerInterval[StorkAppBkpVolResize][9] = 3 * baseInterval
	triggerInterval[StorkAppBkpVolResize][8] = 6 * baseInterval
	triggerInterval[StorkAppBkpVolResize][7] = 9 * baseInterval
	triggerInterval[StorkAppBkpVolResize][6] = 12 * baseInterval
	triggerInterval[StorkAppBkpVolResize][5] = 15 * baseInterval
	triggerInterval[StorkAppBkpVolResize][4] = 18 * baseInterval
	triggerInterval[StorkAppBkpVolResize][3] = 21 * baseInterval
	triggerInterval[StorkAppBkpVolResize][2] = 24 * baseInterval
	triggerInterval[StorkAppBkpVolResize][1] = 27 * baseInterval

	triggerInterval[StorkAppBkpHaUpdate][10] = 1 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][9] = 3 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][8] = 6 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][7] = 9 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][6] = 12 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][5] = 15 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][4] = 18 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][3] = 21 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][2] = 24 * baseInterval
	triggerInterval[StorkAppBkpHaUpdate][1] = 27 * baseInterval

	triggerInterval[StorkAppBkpPxRestart][10] = 1 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][9] = 3 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][8] = 6 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][7] = 9 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][6] = 12 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][5] = 15 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][4] = 18 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][3] = 21 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][2] = 24 * baseInterval
	triggerInterval[StorkAppBkpPxRestart][1] = 27 * baseInterval

	triggerInterval[StorkAppBkpPoolResize][10] = 1 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][9] = 3 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][8] = 6 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][7] = 9 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][6] = 12 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][5] = 15 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][4] = 18 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][3] = 21 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][2] = 24 * baseInterval
	triggerInterval[StorkAppBkpPoolResize][1] = 27 * baseInterval

	baseInterval = 60 * time.Minute

	triggerInterval[AppTasksDown][10] = 1 * baseInterval
	triggerInterval[AppTasksDown][9] = 2 * baseInterval
	triggerInterval[AppTasksDown][8] = 3 * baseInterval
	triggerInterval[AppTasksDown][7] = 4 * baseInterval
	triggerInterval[AppTasksDown][6] = 5 * baseInterval
	triggerInterval[AppTasksDown][5] = 6 * baseInterval
	triggerInterval[AppTasksDown][4] = 7 * baseInterval
	triggerInterval[AppTasksDown][3] = 8 * baseInterval
	triggerInterval[AppTasksDown][2] = 9 * baseInterval
	triggerInterval[AppTasksDown][1] = 10 * baseInterval

	triggerInterval[RebootNode][10] = 1 * baseInterval
	triggerInterval[RebootNode][9] = 3 * baseInterval
	triggerInterval[RebootNode][8] = 6 * baseInterval
	triggerInterval[RebootNode][7] = 9 * baseInterval
	triggerInterval[RebootNode][6] = 12 * baseInterval
	triggerInterval[RebootNode][5] = 15 * baseInterval
	triggerInterval[RebootNode][4] = 18 * baseInterval
	triggerInterval[RebootNode][3] = 21 * baseInterval
	triggerInterval[RebootNode][2] = 24 * baseInterval
	triggerInterval[RebootNode][1] = 27 * baseInterval

	triggerInterval[RebootManyNodes][10] = 1 * baseInterval
	triggerInterval[RebootManyNodes][9] = 3 * baseInterval
	triggerInterval[RebootManyNodes][8] = 6 * baseInterval
	triggerInterval[RebootManyNodes][7] = 9 * baseInterval
	triggerInterval[RebootManyNodes][6] = 12 * baseInterval
	triggerInterval[RebootManyNodes][5] = 15 * baseInterval
	triggerInterval[RebootManyNodes][4] = 18 * baseInterval
	triggerInterval[RebootManyNodes][3] = 21 * baseInterval
	triggerInterval[RebootManyNodes][2] = 24 * baseInterval
	triggerInterval[RebootManyNodes][1] = 27 * baseInterval

	triggerInterval[CrashNode][10] = 1 * baseInterval
	triggerInterval[CrashNode][9] = 3 * baseInterval
	triggerInterval[CrashNode][8] = 6 * baseInterval
	triggerInterval[CrashNode][7] = 9 * baseInterval
	triggerInterval[CrashNode][6] = 12 * baseInterval
	triggerInterval[CrashNode][5] = 15 * baseInterval
	triggerInterval[CrashNode][4] = 18 * baseInterval
	triggerInterval[CrashNode][3] = 21 * baseInterval
	triggerInterval[CrashNode][2] = 24 * baseInterval
	triggerInterval[CrashNode][1] = 27 * baseInterval

	triggerInterval[CrashVolDriver][10] = 1 * baseInterval
	triggerInterval[CrashVolDriver][9] = 3 * baseInterval
	triggerInterval[CrashVolDriver][8] = 6 * baseInterval
	triggerInterval[CrashVolDriver][7] = 9 * baseInterval
	triggerInterval[CrashVolDriver][6] = 12 * baseInterval
	triggerInterval[CrashVolDriver][5] = 15 * baseInterval
	triggerInterval[CrashVolDriver][4] = 18 * baseInterval
	triggerInterval[CrashVolDriver][3] = 21 * baseInterval
	triggerInterval[CrashVolDriver][2] = 24 * baseInterval
	triggerInterval[CrashVolDriver][1] = 27 * baseInterval

	triggerInterval[RestartVolDriver][10] = 1 * baseInterval
	triggerInterval[RestartVolDriver][9] = 3 * baseInterval
	triggerInterval[RestartVolDriver][8] = 6 * baseInterval
	triggerInterval[RestartVolDriver][7] = 9 * baseInterval
	triggerInterval[RestartVolDriver][6] = 12 * baseInterval
	triggerInterval[RestartVolDriver][5] = 15 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[RestartVolDriver][4] = 18 * baseInterval
	triggerInterval[RestartVolDriver][3] = 21 * baseInterval
	triggerInterval[RestartVolDriver][2] = 24 * baseInterval
	triggerInterval[RestartVolDriver][1] = 27 * baseInterval

	triggerInterval[RestartManyVolDriver][10] = 1 * baseInterval
	triggerInterval[RestartManyVolDriver][9] = 3 * baseInterval
	triggerInterval[RestartManyVolDriver][8] = 6 * baseInterval
	triggerInterval[RestartManyVolDriver][7] = 9 * baseInterval
	triggerInterval[RestartManyVolDriver][6] = 12 * baseInterval
	triggerInterval[RestartManyVolDriver][5] = 15 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[RestartManyVolDriver][4] = 18 * baseInterval
	triggerInterval[RestartManyVolDriver][3] = 21 * baseInterval
	triggerInterval[RestartManyVolDriver][2] = 24 * baseInterval
	triggerInterval[RestartManyVolDriver][1] = 27 * baseInterval

	triggerInterval[RestartKvdbVolDriver][10] = 1 * baseInterval
	triggerInterval[RestartKvdbVolDriver][9] = 3 * baseInterval
	triggerInterval[RestartKvdbVolDriver][8] = 6 * baseInterval
	triggerInterval[RestartKvdbVolDriver][7] = 9 * baseInterval
	triggerInterval[RestartKvdbVolDriver][6] = 12 * baseInterval
	triggerInterval[RestartKvdbVolDriver][5] = 15 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[RestartKvdbVolDriver][4] = 18 * baseInterval
	triggerInterval[RestartKvdbVolDriver][3] = 21 * baseInterval
	triggerInterval[RestartKvdbVolDriver][2] = 24 * baseInterval
	triggerInterval[RestartKvdbVolDriver][1] = 27 * baseInterval

	triggerInterval[AppTaskDown][10] = 1 * baseInterval
	triggerInterval[AppTaskDown][9] = 3 * baseInterval
	triggerInterval[AppTaskDown][8] = 6 * baseInterval
	triggerInterval[AppTaskDown][7] = 9 * baseInterval
	triggerInterval[AppTaskDown][6] = 12 * baseInterval
	triggerInterval[AppTaskDown][5] = 15 * baseInterval // Default global chaos level, 1 hr
	triggerInterval[AppTaskDown][4] = 18 * baseInterval
	triggerInterval[AppTaskDown][3] = 21 * baseInterval
	triggerInterval[AppTaskDown][2] = 24 * baseInterval
	triggerInterval[AppTaskDown][1] = 27 * baseInterval

	triggerInterval[HAIncrease][10] = 1 * baseInterval
	triggerInterval[HAIncrease][9] = 3 * baseInterval
	triggerInterval[HAIncrease][8] = 6 * baseInterval
	triggerInterval[HAIncrease][7] = 9 * baseInterval
	triggerInterval[HAIncrease][6] = 12 * baseInterval
	triggerInterval[HAIncrease][5] = 15 * baseInterval // Default global chaos level, 1.5 hrs
	triggerInterval[HAIncrease][4] = 18 * baseInterval
	triggerInterval[HAIncrease][3] = 21 * baseInterval
	triggerInterval[HAIncrease][2] = 24 * baseInterval
	triggerInterval[HAIncrease][1] = 27 * baseInterval

	triggerInterval[HADecrease][10] = 1 * baseInterval
	triggerInterval[HADecrease][9] = 3 * baseInterval
	triggerInterval[HADecrease][8] = 6 * baseInterval
	triggerInterval[HADecrease][7] = 9 * baseInterval
	triggerInterval[HADecrease][6] = 12 * baseInterval
	triggerInterval[HADecrease][5] = 15 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[HADecrease][4] = 18 * baseInterval
	triggerInterval[HADecrease][3] = 21 * baseInterval
	triggerInterval[HADecrease][2] = 24 * baseInterval
	triggerInterval[HADecrease][1] = 27 * baseInterval

	triggerInterval[VolumeClone][10] = 1 * baseInterval
	triggerInterval[VolumeClone][9] = 3 * baseInterval
	triggerInterval[VolumeClone][8] = 6 * baseInterval
	triggerInterval[VolumeClone][7] = 9 * baseInterval
	triggerInterval[VolumeClone][6] = 12 * baseInterval
	triggerInterval[VolumeClone][5] = 15 * baseInterval
	triggerInterval[VolumeClone][4] = 18 * baseInterval
	triggerInterval[VolumeClone][3] = 21 * baseInterval
	triggerInterval[VolumeClone][2] = 24 * baseInterval
	triggerInterval[VolumeClone][1] = 27 * baseInterval

	triggerInterval[VolumeResize][10] = 1 * baseInterval
	triggerInterval[VolumeResize][9] = 3 * baseInterval
	triggerInterval[VolumeResize][8] = 6 * baseInterval
	triggerInterval[VolumeResize][7] = 9 * baseInterval
	triggerInterval[VolumeResize][6] = 12 * baseInterval
	triggerInterval[VolumeResize][5] = 15 * baseInterval
	triggerInterval[VolumeResize][4] = 18 * baseInterval
	triggerInterval[VolumeResize][3] = 21 * baseInterval
	triggerInterval[VolumeResize][2] = 24 * baseInterval
	triggerInterval[VolumeResize][1] = 27 * baseInterval

	triggerInterval[BackupDeleteBackupPod][10] = 1 * baseInterval
	triggerInterval[BackupDeleteBackupPod][9] = 2 * baseInterval
	triggerInterval[BackupDeleteBackupPod][8] = 3 * baseInterval
	triggerInterval[BackupDeleteBackupPod][7] = 4 * baseInterval
	triggerInterval[BackupDeleteBackupPod][6] = 5 * baseInterval
	triggerInterval[BackupDeleteBackupPod][5] = 6 * baseInterval // Default global chaos level, 1 hr

	triggerInterval[CloudSnapShot][10] = 1 * baseInterval
	triggerInterval[CloudSnapShot][9] = 3 * baseInterval
	triggerInterval[CloudSnapShot][8] = 6 * baseInterval
	triggerInterval[CloudSnapShot][7] = 9 * baseInterval
	triggerInterval[CloudSnapShot][6] = 12 * baseInterval
	triggerInterval[CloudSnapShot][5] = 15 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[CloudSnapShot][4] = 18 * baseInterval
	triggerInterval[CloudSnapShot][3] = 21 * baseInterval
	triggerInterval[CloudSnapShot][2] = 24 * baseInterval
	triggerInterval[CloudSnapShot][1] = 27 * baseInterval

	triggerInterval[LocalSnapShot][10] = 1 * baseInterval
	triggerInterval[LocalSnapShot][9] = 3 * baseInterval
	triggerInterval[LocalSnapShot][8] = 6 * baseInterval
	triggerInterval[LocalSnapShot][7] = 9 * baseInterval
	triggerInterval[LocalSnapShot][6] = 12 * baseInterval
	triggerInterval[LocalSnapShot][5] = 15 * baseInterval
	triggerInterval[LocalSnapShot][4] = 18 * baseInterval
	triggerInterval[LocalSnapShot][3] = 21 * baseInterval
	triggerInterval[LocalSnapShot][2] = 24 * baseInterval
	triggerInterval[LocalSnapShot][1] = 27 * baseInterval

	triggerInterval[DeleteLocalSnapShot][10] = 1 * baseInterval
	triggerInterval[DeleteLocalSnapShot][9] = 3 * baseInterval
	triggerInterval[DeleteLocalSnapShot][8] = 6 * baseInterval
	triggerInterval[DeleteLocalSnapShot][7] = 9 * baseInterval
	triggerInterval[DeleteLocalSnapShot][6] = 12 * baseInterval
	triggerInterval[DeleteLocalSnapShot][5] = 15 * baseInterval
	triggerInterval[DeleteLocalSnapShot][4] = 18 * baseInterval
	triggerInterval[DeleteLocalSnapShot][3] = 21 * baseInterval
	triggerInterval[DeleteLocalSnapShot][2] = 24 * baseInterval
	triggerInterval[DeleteLocalSnapShot][1] = 27 * baseInterval

	triggerInterval[EmailReporter][10] = 1 * baseInterval
	triggerInterval[EmailReporter][9] = 2 * baseInterval
	triggerInterval[EmailReporter][8] = 3 * baseInterval
	triggerInterval[EmailReporter][7] = 4 * baseInterval
	triggerInterval[EmailReporter][6] = 5 * baseInterval
	triggerInterval[EmailReporter][5] = 6 * baseInterval
	triggerInterval[EmailReporter][4] = 7 * baseInterval
	triggerInterval[EmailReporter][3] = 8 * baseInterval
	triggerInterval[EmailReporter][2] = 9 * baseInterval
	triggerInterval[EmailReporter][1] = 10 * baseInterval

	triggerInterval[CoreChecker][10] = 1 * baseInterval
	triggerInterval[CoreChecker][9] = 2 * baseInterval
	triggerInterval[CoreChecker][8] = 3 * baseInterval
	triggerInterval[CoreChecker][7] = 4 * baseInterval
	triggerInterval[CoreChecker][6] = 5 * baseInterval
	triggerInterval[CoreChecker][5] = 6 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[CoreChecker][4] = 7 * baseInterval
	triggerInterval[CoreChecker][3] = 8 * baseInterval
	triggerInterval[CoreChecker][2] = 9 * baseInterval
	triggerInterval[CoreChecker][1] = 10 * baseInterval

	triggerInterval[ValidatePdsApps][10] = 1 * baseInterval
	triggerInterval[ValidatePdsApps][9] = 2 * baseInterval
	triggerInterval[ValidatePdsApps][8] = 3 * baseInterval
	triggerInterval[ValidatePdsApps][7] = 4 * baseInterval
	triggerInterval[ValidatePdsApps][6] = 5 * baseInterval
	triggerInterval[ValidatePdsApps][5] = 6 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[ValidatePdsApps][4] = 7 * baseInterval
	triggerInterval[ValidatePdsApps][3] = 8 * baseInterval
	triggerInterval[ValidatePdsApps][2] = 9 * baseInterval
	triggerInterval[ValidatePdsApps][1] = 10 * baseInterval

	triggerInterval[DeployApps][10] = 1 * baseInterval
	triggerInterval[DeployApps][9] = 2 * baseInterval
	triggerInterval[DeployApps][8] = 3 * baseInterval
	triggerInterval[DeployApps][7] = 4 * baseInterval
	triggerInterval[DeployApps][6] = 5 * baseInterval
	triggerInterval[DeployApps][5] = 6 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[DeployApps][4] = 7 * baseInterval
	triggerInterval[DeployApps][3] = 8 * baseInterval
	triggerInterval[DeployApps][2] = 9 * baseInterval
	triggerInterval[DeployApps][1] = 10 * baseInterval

	triggerInterval[PoolAddDisk][10] = 1 * baseInterval
	triggerInterval[PoolAddDisk][9] = 3 * baseInterval
	triggerInterval[PoolAddDisk][8] = 6 * baseInterval
	triggerInterval[PoolAddDisk][7] = 9 * baseInterval
	triggerInterval[PoolAddDisk][6] = 12 * baseInterval
	triggerInterval[PoolAddDisk][5] = 15 * baseInterval
	triggerInterval[PoolAddDisk][4] = 18 * baseInterval
	triggerInterval[PoolAddDisk][3] = 21 * baseInterval
	triggerInterval[PoolAddDisk][2] = 24 * baseInterval
	triggerInterval[PoolAddDisk][1] = 30 * baseInterval

	triggerInterval[PoolResizeDisk][10] = 1 * baseInterval
	triggerInterval[PoolResizeDisk][9] = 3 * baseInterval
	triggerInterval[PoolResizeDisk][8] = 6 * baseInterval
	triggerInterval[PoolResizeDisk][7] = 9 * baseInterval
	triggerInterval[PoolResizeDisk][6] = 12 * baseInterval
	triggerInterval[PoolResizeDisk][5] = 15 * baseInterval
	triggerInterval[PoolResizeDisk][4] = 18 * baseInterval
	triggerInterval[PoolResizeDisk][3] = 21 * baseInterval
	triggerInterval[PoolResizeDisk][2] = 24 * baseInterval
	triggerInterval[PoolResizeDisk][1] = 30 * baseInterval

	triggerInterval[AutoFsTrim][10] = 1 * baseInterval
	triggerInterval[AutoFsTrim][9] = 3 * baseInterval
	triggerInterval[AutoFsTrim][8] = 6 * baseInterval
	triggerInterval[AutoFsTrim][7] = 9 * baseInterval
	triggerInterval[AutoFsTrim][6] = 12 * baseInterval
	triggerInterval[AutoFsTrim][5] = 15 * baseInterval
	triggerInterval[AutoFsTrim][4] = 18 * baseInterval
	triggerInterval[AutoFsTrim][3] = 21 * baseInterval
	triggerInterval[AutoFsTrim][2] = 24 * baseInterval
	triggerInterval[AutoFsTrim][1] = 27 * baseInterval

	triggerInterval[UpdateVolume][10] = 1 * baseInterval
	triggerInterval[UpdateVolume][9] = 3 * baseInterval
	triggerInterval[UpdateVolume][8] = 6 * baseInterval
	triggerInterval[UpdateVolume][7] = 9 * baseInterval
	triggerInterval[UpdateVolume][6] = 12 * baseInterval
	triggerInterval[UpdateVolume][5] = 15 * baseInterval
	triggerInterval[UpdateVolume][4] = 18 * baseInterval
	triggerInterval[UpdateVolume][3] = 21 * baseInterval
	triggerInterval[UpdateVolume][2] = 24 * baseInterval
	triggerInterval[UpdateVolume][1] = 27 * baseInterval

	triggerInterval[NodeDecommission][10] = 1 * baseInterval
	triggerInterval[NodeDecommission][9] = 3 * baseInterval
	triggerInterval[NodeDecommission][8] = 6 * baseInterval
	triggerInterval[NodeDecommission][7] = 9 * baseInterval
	triggerInterval[NodeDecommission][6] = 12 * baseInterval
	triggerInterval[NodeDecommission][5] = 15 * baseInterval
	triggerInterval[NodeDecommission][4] = 18 * baseInterval
	triggerInterval[NodeDecommission][3] = 21 * baseInterval
	triggerInterval[NodeDecommission][2] = 24 * baseInterval
	triggerInterval[NodeDecommission][1] = 27 * baseInterval

	triggerInterval[NodeRejoin][10] = 1 * baseInterval
	triggerInterval[NodeRejoin][9] = 3 * baseInterval
	triggerInterval[NodeRejoin][8] = 6 * baseInterval
	triggerInterval[NodeRejoin][7] = 9 * baseInterval
	triggerInterval[NodeRejoin][6] = 12 * baseInterval
	triggerInterval[NodeRejoin][5] = 15 * baseInterval
	triggerInterval[NodeRejoin][4] = 18 * baseInterval
	triggerInterval[NodeRejoin][3] = 21 * baseInterval
	triggerInterval[NodeRejoin][2] = 24 * baseInterval
	triggerInterval[NodeRejoin][1] = 27 * baseInterval

	triggerInterval[CsiSnapShot][10] = 1 * baseInterval
	triggerInterval[CsiSnapShot][9] = 3 * baseInterval
	triggerInterval[CsiSnapShot][8] = 6 * baseInterval
	triggerInterval[CsiSnapShot][7] = 9 * baseInterval
	triggerInterval[CsiSnapShot][6] = 12 * baseInterval
	triggerInterval[CsiSnapShot][5] = 15 * baseInterval
	triggerInterval[CsiSnapShot][4] = 18 * baseInterval
	triggerInterval[CsiSnapShot][3] = 21 * baseInterval
	triggerInterval[CsiSnapShot][2] = 24 * baseInterval
	triggerInterval[CsiSnapShot][1] = 27 * baseInterval

	triggerInterval[HAIncreaseAndReboot][10] = 1 * baseInterval
	triggerInterval[HAIncreaseAndReboot][9] = 3 * baseInterval
	triggerInterval[HAIncreaseAndReboot][8] = 6 * baseInterval
	triggerInterval[HAIncreaseAndReboot][7] = 9 * baseInterval
	triggerInterval[HAIncreaseAndReboot][6] = 12 * baseInterval
	triggerInterval[HAIncreaseAndReboot][5] = 15 * baseInterval // Default global chaos level, 3 hrs
	triggerInterval[HAIncreaseAndReboot][4] = 18 * baseInterval
	triggerInterval[HAIncreaseAndReboot][3] = 21 * baseInterval
	triggerInterval[HAIncreaseAndReboot][2] = 24 * baseInterval
	triggerInterval[HAIncreaseAndReboot][1] = 27 * baseInterval

	triggerInterval[AddDiskAndReboot][10] = 1 * baseInterval
	triggerInterval[AddDiskAndReboot][9] = 3 * baseInterval
	triggerInterval[AddDiskAndReboot][8] = 6 * baseInterval
	triggerInterval[AddDiskAndReboot][7] = 9 * baseInterval
	triggerInterval[AddDiskAndReboot][6] = 12 * baseInterval
	triggerInterval[AddDiskAndReboot][5] = 15 * baseInterval
	triggerInterval[AddDiskAndReboot][4] = 18 * baseInterval
	triggerInterval[AddDiskAndReboot][3] = 21 * baseInterval
	triggerInterval[AddDiskAndReboot][2] = 24 * baseInterval
	triggerInterval[AddDiskAndReboot][1] = 30 * baseInterval

	triggerInterval[ResizeDiskAndReboot][10] = 1 * baseInterval
	triggerInterval[ResizeDiskAndReboot][9] = 3 * baseInterval
	triggerInterval[ResizeDiskAndReboot][8] = 6 * baseInterval
	triggerInterval[ResizeDiskAndReboot][7] = 9 * baseInterval
	triggerInterval[ResizeDiskAndReboot][6] = 12 * baseInterval
	triggerInterval[ResizeDiskAndReboot][5] = 15 * baseInterval
	triggerInterval[ResizeDiskAndReboot][4] = 18 * baseInterval
	triggerInterval[ResizeDiskAndReboot][3] = 21 * baseInterval
	triggerInterval[ResizeDiskAndReboot][2] = 24 * baseInterval
	triggerInterval[ResizeDiskAndReboot][1] = 30 * baseInterval

	triggerInterval[AutopilotRebalance][10] = 1 * baseInterval
	triggerInterval[AutopilotRebalance][9] = 3 * baseInterval
	triggerInterval[AutopilotRebalance][8] = 6 * baseInterval
	triggerInterval[AutopilotRebalance][7] = 9 * baseInterval
	triggerInterval[AutopilotRebalance][6] = 12 * baseInterval
	triggerInterval[AutopilotRebalance][5] = 15 * baseInterval
	triggerInterval[AutopilotRebalance][4] = 18 * baseInterval
	triggerInterval[AutopilotRebalance][3] = 21 * baseInterval
	triggerInterval[AutopilotRebalance][2] = 24 * baseInterval
	triggerInterval[AutopilotRebalance][1] = 27 * baseInterval

	triggerInterval[VolumeCreatePxRestart][10] = 1 * baseInterval
	triggerInterval[VolumeCreatePxRestart][9] = 2 * baseInterval
	triggerInterval[VolumeCreatePxRestart][8] = 3 * baseInterval
	triggerInterval[VolumeCreatePxRestart][7] = 4 * baseInterval
	triggerInterval[VolumeCreatePxRestart][6] = 5 * baseInterval
	triggerInterval[VolumeCreatePxRestart][5] = 6 * baseInterval
	triggerInterval[VolumeCreatePxRestart][4] = 7 * baseInterval
	triggerInterval[VolumeCreatePxRestart][3] = 8 * baseInterval
	triggerInterval[VolumeCreatePxRestart][2] = 9 * baseInterval
	triggerInterval[VolumeCreatePxRestart][1] = 10 * baseInterval

	baseInterval = 300 * time.Minute

	triggerInterval[UpgradeStork][10] = 1 * baseInterval
	triggerInterval[UpgradeStork][9] = 2 * baseInterval
	triggerInterval[UpgradeStork][8] = 3 * baseInterval
	triggerInterval[UpgradeStork][7] = 4 * baseInterval
	triggerInterval[UpgradeStork][6] = 5 * baseInterval
	triggerInterval[UpgradeStork][5] = 6 * baseInterval

	triggerInterval[UpgradeVolumeDriver][10] = 1 * baseInterval
	triggerInterval[UpgradeVolumeDriver][9] = 2 * baseInterval
	triggerInterval[UpgradeVolumeDriver][8] = 3 * baseInterval
	triggerInterval[UpgradeVolumeDriver][7] = 4 * baseInterval
	triggerInterval[UpgradeVolumeDriver][6] = 5 * baseInterval
	triggerInterval[UpgradeVolumeDriver][5] = 6 * baseInterval

	triggerInterval[KVDBFailover][10] = 1 * baseInterval
	triggerInterval[KVDBFailover][9] = 2 * baseInterval
	triggerInterval[KVDBFailover][8] = 3 * baseInterval
	triggerInterval[KVDBFailover][7] = 4 * baseInterval
	triggerInterval[KVDBFailover][6] = 5 * baseInterval
	triggerInterval[KVDBFailover][5] = 6 * baseInterval

	triggerInterval[VolumesDelete][10] = 1 * baseInterval
	triggerInterval[VolumesDelete][9] = 3 * baseInterval
	triggerInterval[VolumesDelete][8] = 6 * baseInterval
	triggerInterval[VolumesDelete][7] = 9 * baseInterval
	triggerInterval[VolumesDelete][6] = 12 * baseInterval
	triggerInterval[VolumesDelete][5] = 15 * baseInterval
	triggerInterval[VolumesDelete][4] = 18 * baseInterval
	triggerInterval[VolumesDelete][3] = 21 * baseInterval
	triggerInterval[VolumesDelete][2] = 24 * baseInterval
	triggerInterval[VolumesDelete][1] = 27 * baseInterval

	triggerInterval[RelaxedReclaim][10] = 1 * baseInterval
	triggerInterval[RelaxedReclaim][9] = 3 * baseInterval
	triggerInterval[RelaxedReclaim][8] = 6 * baseInterval
	triggerInterval[RelaxedReclaim][7] = 9 * baseInterval
	triggerInterval[RelaxedReclaim][6] = 12 * baseInterval
	triggerInterval[RelaxedReclaim][5] = 15 * baseInterval
	triggerInterval[RelaxedReclaim][4] = 18 * baseInterval
	triggerInterval[RelaxedReclaim][3] = 21 * baseInterval
	triggerInterval[RelaxedReclaim][2] = 24 * baseInterval
	triggerInterval[RelaxedReclaim][1] = 27 * baseInterval

	triggerInterval[Trashcan][10] = 1 * baseInterval
	triggerInterval[Trashcan][9] = 3 * baseInterval
	triggerInterval[Trashcan][8] = 6 * baseInterval
	triggerInterval[Trashcan][7] = 9 * baseInterval
	triggerInterval[Trashcan][6] = 12 * baseInterval
	triggerInterval[Trashcan][5] = 15 * baseInterval
	triggerInterval[Trashcan][4] = 18 * baseInterval
	triggerInterval[Trashcan][3] = 21 * baseInterval
	triggerInterval[Trashcan][2] = 24 * baseInterval
	triggerInterval[Trashcan][1] = 27 * baseInterval

	triggerInterval[CsiSnapRestore][10] = 1 * baseInterval
	triggerInterval[CsiSnapRestore][9] = 3 * baseInterval
	triggerInterval[CsiSnapRestore][8] = 6 * baseInterval
	triggerInterval[CsiSnapRestore][7] = 9 * baseInterval
	triggerInterval[CsiSnapRestore][6] = 12 * baseInterval
	triggerInterval[CsiSnapRestore][5] = 15 * baseInterval
	triggerInterval[CsiSnapRestore][4] = 18 * baseInterval
	triggerInterval[CsiSnapRestore][3] = 21 * baseInterval
	triggerInterval[CsiSnapRestore][2] = 24 * baseInterval
	triggerInterval[CsiSnapRestore][1] = 27 * baseInterval

	triggerInterval[ValidateDeviceMapper][10] = 1 * baseInterval
	triggerInterval[ValidateDeviceMapper][9] = 3 * baseInterval
	triggerInterval[ValidateDeviceMapper][8] = 6 * baseInterval
	triggerInterval[ValidateDeviceMapper][7] = 9 * baseInterval
	triggerInterval[ValidateDeviceMapper][6] = 12 * baseInterval
	triggerInterval[ValidateDeviceMapper][5] = 15 * baseInterval
	triggerInterval[ValidateDeviceMapper][4] = 18 * baseInterval
	triggerInterval[ValidateDeviceMapper][3] = 21 * baseInterval
	triggerInterval[ValidateDeviceMapper][2] = 24 * baseInterval
	triggerInterval[ValidateDeviceMapper][1] = 27 * baseInterval

	triggerInterval[AddDrive][10] = 1 * baseInterval
	triggerInterval[AddDrive][9] = 2 * baseInterval
	triggerInterval[AddDrive][8] = 3 * baseInterval
	triggerInterval[AddDrive][7] = 4 * baseInterval
	triggerInterval[AddDrive][6] = 5 * baseInterval
	triggerInterval[AddDrive][5] = 6 * baseInterval

	// DeleteOldNamespaces trigger will be triggered every 10 hours
	triggerInterval[DeleteOldNamespaces][10] = 2 * baseInterval

	// Chaos Level of 0 means disable test trigger
	triggerInterval[DeployApps][0] = 0
	triggerInterval[RebootNode][0] = 0
	triggerInterval[ValidatePdsApps][0] = 0
	triggerInterval[CrashNode][0] = 0
	triggerInterval[CrashVolDriver][0] = 0
	triggerInterval[HAIncrease][0] = 0
	triggerInterval[HADecrease][0] = 0
	triggerInterval[RestartVolDriver][0] = 0
	triggerInterval[RestartKvdbVolDriver][0] = 0
	triggerInterval[AppTaskDown][0] = 0
	triggerInterval[VolumeClone][0] = 0
	triggerInterval[VolumeResize][0] = 0
	triggerInterval[PoolResizeDisk][0] = 0
	triggerInterval[PoolAddDisk][0] = 0
	triggerInterval[BackupAllApps][0] = 0
	triggerInterval[BackupScheduleAll][0] = 0
	triggerInterval[BackupSpecificResource][0] = 0
	triggerInterval[EmailReporter][0] = 0
	triggerInterval[BackupSpecificResourceOnCluster][0] = 0
	triggerInterval[TestInspectRestore][0] = 0
	triggerInterval[TestInspectBackup][0] = 0
	triggerInterval[TestDeleteBackup][0] = 0
	triggerInterval[RestoreNamespace][0] = 0
	triggerInterval[BackupUsingLabelOnCluster][0] = 0
	triggerInterval[BackupRestartPX][0] = 0
	triggerInterval[BackupRestartNode][0] = 0
	triggerInterval[BackupDeleteBackupPod][0] = 0
	triggerInterval[BackupScaleMongo][0] = 0
	triggerInterval[CloudSnapShot][0] = 0
	triggerInterval[UpgradeStork][0] = 0
	triggerInterval[VolumesDelete][0] = 0
	triggerInterval[LocalSnapShot][0] = 0
	triggerInterval[DeleteLocalSnapShot][0] = 0
	triggerInterval[AppTasksDown][0] = 0
	triggerInterval[AutoFsTrim][0] = 0
	triggerInterval[UpdateVolume][0] = 0
	triggerInterval[RestartManyVolDriver][0] = 0
	triggerInterval[RebootManyNodes][0] = 0
	triggerInterval[CsiSnapShot][0] = 0
	triggerInterval[CsiSnapRestore][0] = 0
	triggerInterval[RelaxedReclaim][0] = 0
	triggerInterval[KVDBFailover][0] = 0
	triggerInterval[ValidateDeviceMapper][0] = 0
	triggerInterval[AsyncDR][0] = 0
	triggerInterval[MetroDR][0] = 0
	triggerInterval[MetroDRMigrationSchedule][0] = 0
	triggerInterval[AsyncDRMigrationSchedule][0] = 0
	triggerInterval[AutoFsTrimAsyncDR][0] = 0
	triggerInterval[IopsBwAsyncDR][0] = 0
	triggerInterval[ConfluentAsyncDR][0] = 0
	triggerInterval[AsyncDRVolumeOnly][0] = 0
	triggerInterval[StorkApplicationBackup][0] = 0
	triggerInterval[StorkAppBkpVolResize][0] = 0
	triggerInterval[StorkAppBkpHaUpdate][0] = 0
	triggerInterval[StorkAppBkpPxRestart][0] = 0
	triggerInterval[StorkAppBkpPoolResize][0] = 0
	triggerInterval[DeleteOldNamespaces][0] = 0
	triggerInterval[HAIncreaseAndReboot][0] = 0
	triggerInterval[AddDrive][0] = 0
	triggerInterval[AddDiskAndReboot][0] = 0
	triggerInterval[ResizeDiskAndReboot][0] = 0
	triggerInterval[AutopilotRebalance][0] = 0
	triggerInterval[VolumeCreatePxRestart][0] = 0
}

func isTriggerEnabled(triggerType string) (time.Duration, bool) {
	var chaosLevel int
	var ok bool
	chaosLevel, ok = ChaosMap[triggerType]
	if !ok {
		chaosLevel = Inst().ChaosLevel
		log.Warnf("Chaos level for trigger [%s] not found in chaos map. Using global chaos level [%d]",
			triggerType, Inst().ChaosLevel)
	}
	if triggerInterval[triggerType][chaosLevel] != 0 {
		return triggerInterval[triggerType][chaosLevel], true
	}
	return triggerInterval[triggerType][chaosLevel], false
}
