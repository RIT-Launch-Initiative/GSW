#!/bin/sh
set -eu

host="${INFLUX_HOST:-http://influxdb:8086}"
org="${DOCKER_INFLUXDB_INIT_ORG:?DOCKER_INFLUXDB_INIT_ORG is required}"
bucket="${DOCKER_INFLUXDB_INIT_BUCKET:?DOCKER_INFLUXDB_INIT_BUCKET is required}"
token="${DOCKER_INFLUXDB_INIT_ADMIN_TOKEN:?DOCKER_INFLUXDB_INIT_ADMIN_TOKEN is required}"
database="${GSW_INFLUXDB_V1_DATABASE:-$bucket}"
retention_policy="${GSW_INFLUXDB_V1_RETENTION_POLICY:-autogen}"

echo "Waiting for InfluxDB at ${host}"
until influx ping --host "${host}" >/dev/null 2>&1; do
	sleep 1
done

bucket_id="$(influx bucket list \
	--host "${host}" \
	--org "${org}" \
	--token "${token}" \
	--name "${bucket}" \
	--hide-headers | awk 'NR == 1 { print $1 }')"

if [ -z "${bucket_id}" ]; then
	echo "Unable to resolve bucket ID for ${bucket}" >&2
	exit 1
fi

if influx v1 dbrp list \
	--host "${host}" \
	--org "${org}" \
	--token "${token}" \
	--db "${database}" \
	--hide-headers | grep -Eq "[[:space:]]${retention_policy}([[:space:]]|$)"; then
	echo "DBRP mapping already exists for ${database}/${retention_policy}"
	exit 0
fi

echo "Creating DBRP mapping for ${database}/${retention_policy}"
influx v1 dbrp create \
	--host "${host}" \
	--org "${org}" \
	--token "${token}" \
	--bucket-id "${bucket_id}" \
	--db "${database}" \
	--rp "${retention_policy}" \
	--default
