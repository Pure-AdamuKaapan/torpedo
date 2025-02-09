/*
PDS API

Portworx Data Services API Server

API version: 1.0.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package pds

import (
	"encoding/json"
)

// ModelsDeploymentTargetCapabilities struct for ModelsDeploymentTargetCapabilities
type ModelsDeploymentTargetCapabilities struct {
	Backup *string `json:"backup,omitempty"`
	Capabilities *string `json:"capabilities,omitempty"`
	Cassandra *string `json:"cassandra,omitempty"`
	Consul *string `json:"consul,omitempty"`
	Couchbase *string `json:"couchbase,omitempty"`
	CrdReporting *string `json:"crd_reporting,omitempty"`
	DataServiceTls *string `json:"data_service_tls,omitempty"`
	Database *string `json:"database,omitempty"`
	Elasticsearch *string `json:"elasticsearch,omitempty"`
	Kafka *string `json:"kafka,omitempty"`
	Mongodb *string `json:"mongodb,omitempty"`
	Mysql *string `json:"mysql,omitempty"`
	Postgresql *string `json:"postgresql,omitempty"`
	Rabbitmq *string `json:"rabbitmq,omitempty"`
	Redis *string `json:"redis,omitempty"`
	Sqlserver *string `json:"sqlserver,omitempty"`
	Zookeeper *string `json:"zookeeper,omitempty"`
}

// NewModelsDeploymentTargetCapabilities instantiates a new ModelsDeploymentTargetCapabilities object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewModelsDeploymentTargetCapabilities() *ModelsDeploymentTargetCapabilities {
	this := ModelsDeploymentTargetCapabilities{}
	return &this
}

// NewModelsDeploymentTargetCapabilitiesWithDefaults instantiates a new ModelsDeploymentTargetCapabilities object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewModelsDeploymentTargetCapabilitiesWithDefaults() *ModelsDeploymentTargetCapabilities {
	this := ModelsDeploymentTargetCapabilities{}
	return &this
}

// GetBackup returns the Backup field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetBackup() string {
	if o == nil || o.Backup == nil {
		var ret string
		return ret
	}
	return *o.Backup
}

// GetBackupOk returns a tuple with the Backup field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetBackupOk() (*string, bool) {
	if o == nil || o.Backup == nil {
		return nil, false
	}
	return o.Backup, true
}

// HasBackup returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasBackup() bool {
	if o != nil && o.Backup != nil {
		return true
	}

	return false
}

// SetBackup gets a reference to the given string and assigns it to the Backup field.
func (o *ModelsDeploymentTargetCapabilities) SetBackup(v string) {
	o.Backup = &v
}

// GetCapabilities returns the Capabilities field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetCapabilities() string {
	if o == nil || o.Capabilities == nil {
		var ret string
		return ret
	}
	return *o.Capabilities
}

// GetCapabilitiesOk returns a tuple with the Capabilities field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetCapabilitiesOk() (*string, bool) {
	if o == nil || o.Capabilities == nil {
		return nil, false
	}
	return o.Capabilities, true
}

// HasCapabilities returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasCapabilities() bool {
	if o != nil && o.Capabilities != nil {
		return true
	}

	return false
}

// SetCapabilities gets a reference to the given string and assigns it to the Capabilities field.
func (o *ModelsDeploymentTargetCapabilities) SetCapabilities(v string) {
	o.Capabilities = &v
}

// GetCassandra returns the Cassandra field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetCassandra() string {
	if o == nil || o.Cassandra == nil {
		var ret string
		return ret
	}
	return *o.Cassandra
}

// GetCassandraOk returns a tuple with the Cassandra field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetCassandraOk() (*string, bool) {
	if o == nil || o.Cassandra == nil {
		return nil, false
	}
	return o.Cassandra, true
}

// HasCassandra returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasCassandra() bool {
	if o != nil && o.Cassandra != nil {
		return true
	}

	return false
}

// SetCassandra gets a reference to the given string and assigns it to the Cassandra field.
func (o *ModelsDeploymentTargetCapabilities) SetCassandra(v string) {
	o.Cassandra = &v
}

// GetConsul returns the Consul field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetConsul() string {
	if o == nil || o.Consul == nil {
		var ret string
		return ret
	}
	return *o.Consul
}

// GetConsulOk returns a tuple with the Consul field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetConsulOk() (*string, bool) {
	if o == nil || o.Consul == nil {
		return nil, false
	}
	return o.Consul, true
}

// HasConsul returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasConsul() bool {
	if o != nil && o.Consul != nil {
		return true
	}

	return false
}

// SetConsul gets a reference to the given string and assigns it to the Consul field.
func (o *ModelsDeploymentTargetCapabilities) SetConsul(v string) {
	o.Consul = &v
}

// GetCouchbase returns the Couchbase field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetCouchbase() string {
	if o == nil || o.Couchbase == nil {
		var ret string
		return ret
	}
	return *o.Couchbase
}

// GetCouchbaseOk returns a tuple with the Couchbase field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetCouchbaseOk() (*string, bool) {
	if o == nil || o.Couchbase == nil {
		return nil, false
	}
	return o.Couchbase, true
}

// HasCouchbase returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasCouchbase() bool {
	if o != nil && o.Couchbase != nil {
		return true
	}

	return false
}

// SetCouchbase gets a reference to the given string and assigns it to the Couchbase field.
func (o *ModelsDeploymentTargetCapabilities) SetCouchbase(v string) {
	o.Couchbase = &v
}

// GetCrdReporting returns the CrdReporting field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetCrdReporting() string {
	if o == nil || o.CrdReporting == nil {
		var ret string
		return ret
	}
	return *o.CrdReporting
}

// GetCrdReportingOk returns a tuple with the CrdReporting field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetCrdReportingOk() (*string, bool) {
	if o == nil || o.CrdReporting == nil {
		return nil, false
	}
	return o.CrdReporting, true
}

// HasCrdReporting returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasCrdReporting() bool {
	if o != nil && o.CrdReporting != nil {
		return true
	}

	return false
}

// SetCrdReporting gets a reference to the given string and assigns it to the CrdReporting field.
func (o *ModelsDeploymentTargetCapabilities) SetCrdReporting(v string) {
	o.CrdReporting = &v
}

// GetDataServiceTls returns the DataServiceTls field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetDataServiceTls() string {
	if o == nil || o.DataServiceTls == nil {
		var ret string
		return ret
	}
	return *o.DataServiceTls
}

// GetDataServiceTlsOk returns a tuple with the DataServiceTls field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetDataServiceTlsOk() (*string, bool) {
	if o == nil || o.DataServiceTls == nil {
		return nil, false
	}
	return o.DataServiceTls, true
}

// HasDataServiceTls returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasDataServiceTls() bool {
	if o != nil && o.DataServiceTls != nil {
		return true
	}

	return false
}

// SetDataServiceTls gets a reference to the given string and assigns it to the DataServiceTls field.
func (o *ModelsDeploymentTargetCapabilities) SetDataServiceTls(v string) {
	o.DataServiceTls = &v
}

// GetDatabase returns the Database field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetDatabase() string {
	if o == nil || o.Database == nil {
		var ret string
		return ret
	}
	return *o.Database
}

// GetDatabaseOk returns a tuple with the Database field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetDatabaseOk() (*string, bool) {
	if o == nil || o.Database == nil {
		return nil, false
	}
	return o.Database, true
}

// HasDatabase returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasDatabase() bool {
	if o != nil && o.Database != nil {
		return true
	}

	return false
}

// SetDatabase gets a reference to the given string and assigns it to the Database field.
func (o *ModelsDeploymentTargetCapabilities) SetDatabase(v string) {
	o.Database = &v
}

// GetElasticsearch returns the Elasticsearch field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetElasticsearch() string {
	if o == nil || o.Elasticsearch == nil {
		var ret string
		return ret
	}
	return *o.Elasticsearch
}

// GetElasticsearchOk returns a tuple with the Elasticsearch field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetElasticsearchOk() (*string, bool) {
	if o == nil || o.Elasticsearch == nil {
		return nil, false
	}
	return o.Elasticsearch, true
}

// HasElasticsearch returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasElasticsearch() bool {
	if o != nil && o.Elasticsearch != nil {
		return true
	}

	return false
}

// SetElasticsearch gets a reference to the given string and assigns it to the Elasticsearch field.
func (o *ModelsDeploymentTargetCapabilities) SetElasticsearch(v string) {
	o.Elasticsearch = &v
}

// GetKafka returns the Kafka field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetKafka() string {
	if o == nil || o.Kafka == nil {
		var ret string
		return ret
	}
	return *o.Kafka
}

// GetKafkaOk returns a tuple with the Kafka field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetKafkaOk() (*string, bool) {
	if o == nil || o.Kafka == nil {
		return nil, false
	}
	return o.Kafka, true
}

// HasKafka returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasKafka() bool {
	if o != nil && o.Kafka != nil {
		return true
	}

	return false
}

// SetKafka gets a reference to the given string and assigns it to the Kafka field.
func (o *ModelsDeploymentTargetCapabilities) SetKafka(v string) {
	o.Kafka = &v
}

// GetMongodb returns the Mongodb field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetMongodb() string {
	if o == nil || o.Mongodb == nil {
		var ret string
		return ret
	}
	return *o.Mongodb
}

// GetMongodbOk returns a tuple with the Mongodb field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetMongodbOk() (*string, bool) {
	if o == nil || o.Mongodb == nil {
		return nil, false
	}
	return o.Mongodb, true
}

// HasMongodb returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasMongodb() bool {
	if o != nil && o.Mongodb != nil {
		return true
	}

	return false
}

// SetMongodb gets a reference to the given string and assigns it to the Mongodb field.
func (o *ModelsDeploymentTargetCapabilities) SetMongodb(v string) {
	o.Mongodb = &v
}

// GetMysql returns the Mysql field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetMysql() string {
	if o == nil || o.Mysql == nil {
		var ret string
		return ret
	}
	return *o.Mysql
}

// GetMysqlOk returns a tuple with the Mysql field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetMysqlOk() (*string, bool) {
	if o == nil || o.Mysql == nil {
		return nil, false
	}
	return o.Mysql, true
}

// HasMysql returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasMysql() bool {
	if o != nil && o.Mysql != nil {
		return true
	}

	return false
}

// SetMysql gets a reference to the given string and assigns it to the Mysql field.
func (o *ModelsDeploymentTargetCapabilities) SetMysql(v string) {
	o.Mysql = &v
}

// GetPostgresql returns the Postgresql field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetPostgresql() string {
	if o == nil || o.Postgresql == nil {
		var ret string
		return ret
	}
	return *o.Postgresql
}

// GetPostgresqlOk returns a tuple with the Postgresql field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetPostgresqlOk() (*string, bool) {
	if o == nil || o.Postgresql == nil {
		return nil, false
	}
	return o.Postgresql, true
}

// HasPostgresql returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasPostgresql() bool {
	if o != nil && o.Postgresql != nil {
		return true
	}

	return false
}

// SetPostgresql gets a reference to the given string and assigns it to the Postgresql field.
func (o *ModelsDeploymentTargetCapabilities) SetPostgresql(v string) {
	o.Postgresql = &v
}

// GetRabbitmq returns the Rabbitmq field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetRabbitmq() string {
	if o == nil || o.Rabbitmq == nil {
		var ret string
		return ret
	}
	return *o.Rabbitmq
}

// GetRabbitmqOk returns a tuple with the Rabbitmq field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetRabbitmqOk() (*string, bool) {
	if o == nil || o.Rabbitmq == nil {
		return nil, false
	}
	return o.Rabbitmq, true
}

// HasRabbitmq returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasRabbitmq() bool {
	if o != nil && o.Rabbitmq != nil {
		return true
	}

	return false
}

// SetRabbitmq gets a reference to the given string and assigns it to the Rabbitmq field.
func (o *ModelsDeploymentTargetCapabilities) SetRabbitmq(v string) {
	o.Rabbitmq = &v
}

// GetRedis returns the Redis field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetRedis() string {
	if o == nil || o.Redis == nil {
		var ret string
		return ret
	}
	return *o.Redis
}

// GetRedisOk returns a tuple with the Redis field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetRedisOk() (*string, bool) {
	if o == nil || o.Redis == nil {
		return nil, false
	}
	return o.Redis, true
}

// HasRedis returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasRedis() bool {
	if o != nil && o.Redis != nil {
		return true
	}

	return false
}

// SetRedis gets a reference to the given string and assigns it to the Redis field.
func (o *ModelsDeploymentTargetCapabilities) SetRedis(v string) {
	o.Redis = &v
}

// GetSqlserver returns the Sqlserver field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetSqlserver() string {
	if o == nil || o.Sqlserver == nil {
		var ret string
		return ret
	}
	return *o.Sqlserver
}

// GetSqlserverOk returns a tuple with the Sqlserver field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetSqlserverOk() (*string, bool) {
	if o == nil || o.Sqlserver == nil {
		return nil, false
	}
	return o.Sqlserver, true
}

// HasSqlserver returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasSqlserver() bool {
	if o != nil && o.Sqlserver != nil {
		return true
	}

	return false
}

// SetSqlserver gets a reference to the given string and assigns it to the Sqlserver field.
func (o *ModelsDeploymentTargetCapabilities) SetSqlserver(v string) {
	o.Sqlserver = &v
}

// GetZookeeper returns the Zookeeper field value if set, zero value otherwise.
func (o *ModelsDeploymentTargetCapabilities) GetZookeeper() string {
	if o == nil || o.Zookeeper == nil {
		var ret string
		return ret
	}
	return *o.Zookeeper
}

// GetZookeeperOk returns a tuple with the Zookeeper field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsDeploymentTargetCapabilities) GetZookeeperOk() (*string, bool) {
	if o == nil || o.Zookeeper == nil {
		return nil, false
	}
	return o.Zookeeper, true
}

// HasZookeeper returns a boolean if a field has been set.
func (o *ModelsDeploymentTargetCapabilities) HasZookeeper() bool {
	if o != nil && o.Zookeeper != nil {
		return true
	}

	return false
}

// SetZookeeper gets a reference to the given string and assigns it to the Zookeeper field.
func (o *ModelsDeploymentTargetCapabilities) SetZookeeper(v string) {
	o.Zookeeper = &v
}

func (o ModelsDeploymentTargetCapabilities) MarshalJSON() ([]byte, error) {
	toSerialize := map[string]interface{}{}
	if o.Backup != nil {
		toSerialize["backup"] = o.Backup
	}
	if o.Capabilities != nil {
		toSerialize["capabilities"] = o.Capabilities
	}
	if o.Cassandra != nil {
		toSerialize["cassandra"] = o.Cassandra
	}
	if o.Consul != nil {
		toSerialize["consul"] = o.Consul
	}
	if o.Couchbase != nil {
		toSerialize["couchbase"] = o.Couchbase
	}
	if o.CrdReporting != nil {
		toSerialize["crd_reporting"] = o.CrdReporting
	}
	if o.DataServiceTls != nil {
		toSerialize["data_service_tls"] = o.DataServiceTls
	}
	if o.Database != nil {
		toSerialize["database"] = o.Database
	}
	if o.Elasticsearch != nil {
		toSerialize["elasticsearch"] = o.Elasticsearch
	}
	if o.Kafka != nil {
		toSerialize["kafka"] = o.Kafka
	}
	if o.Mongodb != nil {
		toSerialize["mongodb"] = o.Mongodb
	}
	if o.Mysql != nil {
		toSerialize["mysql"] = o.Mysql
	}
	if o.Postgresql != nil {
		toSerialize["postgresql"] = o.Postgresql
	}
	if o.Rabbitmq != nil {
		toSerialize["rabbitmq"] = o.Rabbitmq
	}
	if o.Redis != nil {
		toSerialize["redis"] = o.Redis
	}
	if o.Sqlserver != nil {
		toSerialize["sqlserver"] = o.Sqlserver
	}
	if o.Zookeeper != nil {
		toSerialize["zookeeper"] = o.Zookeeper
	}
	return json.Marshal(toSerialize)
}

type NullableModelsDeploymentTargetCapabilities struct {
	value *ModelsDeploymentTargetCapabilities
	isSet bool
}

func (v NullableModelsDeploymentTargetCapabilities) Get() *ModelsDeploymentTargetCapabilities {
	return v.value
}

func (v *NullableModelsDeploymentTargetCapabilities) Set(val *ModelsDeploymentTargetCapabilities) {
	v.value = val
	v.isSet = true
}

func (v NullableModelsDeploymentTargetCapabilities) IsSet() bool {
	return v.isSet
}

func (v *NullableModelsDeploymentTargetCapabilities) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableModelsDeploymentTargetCapabilities(val *ModelsDeploymentTargetCapabilities) *NullableModelsDeploymentTargetCapabilities {
	return &NullableModelsDeploymentTargetCapabilities{value: val, isSet: true}
}

func (v NullableModelsDeploymentTargetCapabilities) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableModelsDeploymentTargetCapabilities) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}


