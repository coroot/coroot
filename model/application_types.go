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
	ApplicationTypeKeyDB         ApplicationType = "keydb"
	ApplicationTypeMongodb       ApplicationType = "mongodb"
	ApplicationTypeRabbitmq      ApplicationType = "rabbitmq"
	ApplicationTypeKafka         ApplicationType = "kafka"
	ApplicationTypeZookeeper     ApplicationType = "zookeeper"
	ApplicationTypeRDS           ApplicationType = "aws-rds"
	ApplicationTypeElastiCache   ApplicationType = "aws-elasticache"
	ApplicationTypeNats          ApplicationType = "nats"
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
