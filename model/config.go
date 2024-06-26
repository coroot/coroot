package model

import "golang.org/x/exp/maps"

type Config struct {
	ApplicationInstrumentation []ApplicationInstrumentation `json:"application_instrumentation"`

	AWSConfig *AWSConfig `json:"aws_config"`
}

type AWSConfig struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`

	RDSTagFilters         map[string]string `json:"rds_tag_filters"`
	ElasticacheTagFilters map[string]string `json:"elasticache_tag_filters"`
}

func (c *AWSConfig) Equal(other *AWSConfig) bool {
	return c.Region == other.Region &&
		c.AccessKeyID == other.AccessKeyID &&
		c.SecretAccessKey == other.SecretAccessKey &&
		maps.Equal(c.RDSTagFilters, other.RDSTagFilters) &&
		maps.Equal(c.ElasticacheTagFilters, other.ElasticacheTagFilters)
}
