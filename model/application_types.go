package model

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
	ApplicationTypeMongodb       ApplicationType = "mongodb"
	ApplicationTypeRabbitmq      ApplicationType = "rabbitmq"
	ApplicationTypeKafka         ApplicationType = "kafka"
	ApplicationTypeZookeeper     ApplicationType = "zookeeper"
)

func (at ApplicationType) IsDatabase() bool {
	switch at {
	case ApplicationTypeCassandra, ApplicationTypeMemcached,
		ApplicationTypeZookeeper, ApplicationTypeElasticsearch, ApplicationTypePostgres,
		ApplicationTypeMysql, ApplicationTypeRedis, ApplicationTypeMongodb:
		return true
	}
	return false
}

func (at ApplicationType) IsQueue() bool {
	switch at {
	case ApplicationTypeKafka, ApplicationTypeRabbitmq:
		return true
	}
	return false
}
