package model

import "strings"

type ApplicationType string

const (
	ApplicationTypeUnknown       ApplicationType = ""
	ApplicationTypePostgres      ApplicationType = "postgres"
	ApplicationTypePgbouncer     ApplicationType = "pgbouncer"
	ApplicationTypeMysql         ApplicationType = "mysql"
	ApplicationTypeCassandra     ApplicationType = "cassandra"
	ApplicationTypeElasticsearch ApplicationType = "elasticsearch"
	ApplicationTypeMemcached     ApplicationType = "memcached"
	ApplicationTypeRedis         ApplicationType = "redis"
	ApplicationTypeKeyDB         ApplicationType = "keydb"
	ApplicationTypeMongodb       ApplicationType = "mongodb"
	ApplicationTypeMongos        ApplicationType = "mongos"
	ApplicationTypeRabbitmq      ApplicationType = "rabbitmq"
	ApplicationTypeKafka         ApplicationType = "kafka"
	ApplicationTypeZookeeper     ApplicationType = "zookeeper"
	ApplicationTypeRDS           ApplicationType = "aws-rds"
	ApplicationTypeElastiCache   ApplicationType = "aws-elasticache"
	ApplicationTypeNats          ApplicationType = "nats"
	ApplicationTypeDotNet        ApplicationType = "dotnet"
	ApplicationTypeGolang        ApplicationType = "golang"
	ApplicationTypePHP           ApplicationType = "php"
	ApplicationTypeJava          ApplicationType = "java"
	ApplicationTypeNodeJS        ApplicationType = "nodejs"
	ApplicationTypePython        ApplicationType = "python"
)

func (at ApplicationType) IsDatabase() bool {
	switch at {
	case ApplicationTypeCassandra, ApplicationTypeMemcached,
		ApplicationTypeZookeeper, ApplicationTypeElasticsearch, ApplicationTypePostgres,
		ApplicationTypeMysql, ApplicationTypeRedis, ApplicationTypeKeyDB, ApplicationTypeMongodb:
		return true
	}
	return false
}

func (at ApplicationType) IsQueue() bool {
	switch at {
	case ApplicationTypeKafka, ApplicationTypeRabbitmq, ApplicationTypeNats:
		return true
	}
	return false
}

func (at ApplicationType) IsCredentialsRequired() bool {
	switch at {
	case ApplicationTypePostgres, ApplicationTypeMysql:
		return true
	}
	return false
}

func (at ApplicationType) IsLanguage() bool {
	switch at {
	case ApplicationTypeGolang, ApplicationTypeDotNet, ApplicationTypePHP, ApplicationTypeJava, ApplicationTypeNodeJS:
		return true
	}
	return false
}

func (at ApplicationType) Weight() int {
	switch {
	case at.IsDatabase():
		return 1
	case at.IsQueue():
		return 2
	case at.IsLanguage():
		return 4
	}
	return 3
}

func (at ApplicationType) AuditReport() AuditReportName {
	switch at {
	case ApplicationTypePostgres:
		return AuditReportPostgres
	case ApplicationTypeMysql:
		return AuditReportMysql
	case ApplicationTypeRedis:
		return AuditReportRedis
	case ApplicationTypeMongodb, ApplicationTypeMongos:
		return AuditReportMongodb
	case ApplicationTypeMemcached:
		return AuditReportMemcached
	case ApplicationTypeJava:
		return AuditReportJvm
	case ApplicationTypeDotNet:
		return AuditReportDotNet
	case ApplicationTypePython:
		return AuditReportPython
	}
	return ""
}

func (at ApplicationType) Icon() string {
	switch {
	case strings.HasPrefix(string(at), "kube"):
		return "kubernetes"
	case at == ApplicationTypePgbouncer:
		return "postgres"
	case at == ApplicationTypeMongos:
		return "mongodb"
	}
	return string(at)
}
