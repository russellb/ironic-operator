#!/bin/bash
set -e
# Extract connection details
RABBIT_HOSTNAME=$(echo "${RABBITMQ_ADMIN_CONNECTION}" | \
  awk -F'[@]' '{print $2}' | \
  awk -F'[:/]' '{print $1}')
RABBIT_PORT=$(echo "${RABBITMQ_ADMIN_CONNECTION}" | \
  awk -F'[@]' '{print $2}' | \
  awk -F'[:/]' '{print $2}')

# Extract Admin User credential
RABBITMQ_ADMIN_USERNAME=$(echo "${RABBITMQ_ADMIN_CONNECTION}" | \
  awk -F'[@]' '{print $1}' | \
  awk -F'[//:]' '{print $4}')
RABBITMQ_ADMIN_PASSWORD=$(echo "${RABBITMQ_ADMIN_CONNECTION}" | \
  awk -F'[@]' '{print $1}' | \
  awk -F'[//:]' '{print $5}')

# Extract User credential
RABBITMQ_USERNAME=$(echo "${RABBITMQ_USER_CONNECTION}" | \
  awk -F'[@]' '{print $1}' | \
  awk -F'[//:]' '{print $4}')
RABBITMQ_PASSWORD=$(echo "${RABBITMQ_USER_CONNECTION}" | \
  awk -F'[@]' '{print $1}' | \
  awk -F'[//:]' '{print $5}')

# Extract User vHost
RABBITMQ_VHOST=$(echo "${RABBITMQ_USER_CONNECTION}" | \
  awk -F'[@]' '{print $2}' | \
  awk -F'[:/]' '{print $3}')

function rabbitmqadmin_cli () {
  rabbitmqadmin \
    --host="${RABBIT_HOSTNAME}" \
    --port="${RABBIT_PORT}" \
    --username="${RABBITMQ_ADMIN_USERNAME}" \
    --password="${RABBITMQ_ADMIN_PASSWORD}" \
    ${@}
}

echo "Managing: User: ${RABBITMQ_USERNAME}"
rabbitmqadmin_cli \
  declare user \
  name="${RABBITMQ_USERNAME}" \
  password="${RABBITMQ_PASSWORD}" \
  tags="user"

echo "Managing: vHost: ${RABBITMQ_VHOST}"
rabbitmqadmin_cli \
  declare vhost \
  name="${RABBITMQ_VHOST}"

echo "Managing: Permissions: ${RABBITMQ_USERNAME} on ${RABBITMQ_VHOST}"
rabbitmqadmin_cli \
  declare permission \
  vhost="${RABBITMQ_VHOST}" \
  user="${RABBITMQ_USERNAME}" \
  configure=".*" \
  write=".*" \
  read=".*"

if [ ! -z "$RABBITMQ_AUXILIARY_CONFIGURATION" ]
then
  echo "Applying additional configuration"
  echo "${RABBITMQ_AUXILIARY_CONFIGURATION}" > /tmp/rmq_definitions.json
  rabbitmqadmin_cli import /tmp/rmq_definitions.json
fi
