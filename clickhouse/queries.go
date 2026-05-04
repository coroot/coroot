package clickhouse

import "fmt"

const (
	qSamples = `
SELECT
    StackHash AS hash,
    sum(Value) AS value
FROM @@table_profiling_samples@@
WHERE
    ServiceName IN (@services) AND
    Type = @type AND
    Start < @to AND End > @from AND
    (empty(@containers) OR has(@containers, Labels['container.id'])) AND
	(@namespace = '' OR @namespace = Labels['namespace']) AND
	(@pod = '' OR @pod = Labels['pod'])
GROUP BY StackHash
HAVING value > 0
ORDER BY value DESC
LIMIT 5000
`
	qSamplesDiff = `
SELECT
	StackHash AS hash,
	sum(CASE WHEN End < @middle THEN Value ELSE 0 END) AS base,
	sum(CASE WHEN Start > @middle THEN Value ELSE 0 END) AS comp
FROM @@table_profiling_samples@@
WHERE
    ServiceName IN (@services) AND
    Type = @type AND
    Start < @to AND End > @from AND
    (empty(@containers) OR has(@containers, Labels['container.id'])) AND
	(@namespace = '' OR @namespace = Labels['namespace']) AND
	(@pod = '' OR @pod = Labels['pod'])
GROUP BY StackHash
HAVING base + comp > 0
ORDER BY base + comp DESC
LIMIT 5000
`
	qStacks = `
SELECT
    Hash AS hash,
    any(Stack) AS stack
FROM @@table_profiling_stacks@@
WHERE
    ServiceName IN (@services) AND
    LastSeen > @from AND
    Hash GLOBAL IN (SELECT hash FROM samples)
GROUP BY Hash
`
	qProfiles = `
SELECT 
    count(distinct Start) / greatest(1, count(distinct Labels['container.id'], Labels['namespace'], Labels['pod'])) AS profiles
FROM @@table_profiling_samples@@
WHERE 
    ServiceName IN (@services) AND 
    Type = @type AND 
    Start < @to AND End > @from AND
    (empty(@containers) OR has(@containers, Labels['container.id'])) AND
	(@namespace = '' OR @namespace = Labels['namespace']) AND
	(@pod = '' OR @pod = Labels['pod'])
`
)

var (
	qProfileTypes = "SELECT DISTINCT ServiceName, Type FROM @@table_profiling_profiles@@ WHERE LastSeen >= @from"
	qProfile      = fmt.Sprintf("WITH samples AS (%s), stacks AS (%s) SELECT value, stack FROM stacks JOIN samples USING(hash)", qSamples, qStacks)
	qProfileAvg   = fmt.Sprintf("WITH samples AS (%s), stacks AS (%s), profiles AS (%s) SELECT toInt64(value/profiles), stack FROM stacks JOIN samples USING(hash), profiles", qSamples, qStacks, qProfiles)
	qProfileDiff  = fmt.Sprintf("WITH samples AS (%s), stacks AS (%s) SELECT base, comp, stack FROM stacks JOIN samples USING(hash)", qSamplesDiff, qStacks)
)
