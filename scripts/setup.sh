#!/bin/bash

set -e
CLOUD=$1
CLUSTER=$2
if [ -z "$CLUSTER" ] || [ -z "$CLOUD" ]
then
  echo "Usage: $0 <aws|azure|gcp> <cluster>"
  exit 1
fi

# If rerunning init.sh, uncomment below command to wipe
# and to re-copy scripts dir.
# roachprod run "$CLUSTER" rm -- -rf ./scripts

echo "Uploading scripts to $CLUSTER"
roachprod put "$CLUSTER" `dirname $0` scripts
roachprod run "$CLUSTER" chmod -- -R +x ./scripts

echo "Executing setup.sh on $CLUSTER"
roachprod run "$CLUSTER" sudo ./scripts/gen/setup.sh "$CLOUD"