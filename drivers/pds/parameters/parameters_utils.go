package parameters

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/portworx/sched-ops/k8s/core"
	"github.com/portworx/torpedo/pkg/log"
)

type Parameter struct {
	DataServiceToTest []struct {
		Name          string `json:"Name"`
		Version       string `json:"Version"`
		Image         string `json:"Image"`
		Replicas      int    `json:"Replicas"`
		ScaleReplicas int    `json:"ScaleReplicas"`
		OldVersion    string `json:"OldVersion"`
		OldImage      string `json:"OldImage"`
	} `json:"DataServiceToTest"`
	ForceImageID bool
	InfraToTest  struct {
		ControlPlaneURL string `json:"ControlPlaneURL"`
		AccountName     string `json:"AccountName"`
		TenantName      string `json:"TenantName"`
		ProjectName     string `json:"ProjectName"`
		ClusterType     string `json:"ClusterType"`
		Namespace       string `json:"Namespace"`
		PxNamespace     string `json:"PxNamespace"`
		PDSNamespace    string `json:"PDSNamespace"`
	} `json:"InfraToTest"`
	PDSHelmVersions struct {
		LatestHelmVersion   string `json:"LatestHelmVersion"`
		PreviousHelmVersion string `json:"PreviousHelmVersion"`
	} `json:"PDSHelmVersions"`
	Users struct {
		AdminUsername    string `json:"AdminUsername"`
		AdminPassword    string `json:"AdminPassword"`
		NonAdminUsername string `json:"NonAdminUsername"`
		NonAdminPassword string `json:"NonAdminPassword"`
	} `json:"Users"`
	ResiliencyTest struct {
		CheckTillReplica int32 `json:"CheckTillReplica"`
	} `json:"ResiliencyTest"`
}

const (
	defaultParams      = "../drivers/pds/parameters/pds_default_parameters.json"
	pdsParamsConfigmap = "pds-params"
	configmapNamespace = "default"
)

type Customparams struct{}

// ReadParams reads the params from given or default json
func (customparams *Customparams) ReadParams(filename string) (*Parameter, error) {
	var jsonPara Parameter
	var err error

	if filename == "" {
		filename, err = filepath.Abs(defaultParams)
		log.Infof("filename %v", filename)
		if err != nil {
			return nil, err
		}
		log.Infof("Parameter json file is not used, use initial parameters value.")
		log.InfoD("Reading params from %v ", filename)
		file, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(file, &jsonPara)
		if err != nil {
			return nil, err
		}
	} else {
		cm, err := core.Instance().GetConfigMap(pdsParamsConfigmap, configmapNamespace)
		if err != nil {
			return nil, err
		}
		if len(cm.Data) > 0 {
			configmap := &cm.Data
			for key, data := range *configmap {
				log.InfoD("key %v \n value %v", key, data)
				json_data := []byte(data)
				err = json.Unmarshal(json_data, &jsonPara)
				if err != nil {
					log.FailOnError(err, "Error while unmarshalling json:")
				}
			}
		}
	}
	return &jsonPara, nil
}
