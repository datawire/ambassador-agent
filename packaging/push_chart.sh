#!/bin/bash

set -e

# This will be running in automation so we don't want the
# interactive pager to pop up
export AWS_PAGER=""

# Get the toplevel dir of the repo so we can run this command
# no matter which directory we are in.
TOP_DIR="$( git rev-parse --show-toplevel)"
echo "$TOP_DIR"

tmpdir=$(mktemp -d)

helm package \
    --version=${A8R_AGENT_VERSION#v} \
    --app-version=${A8R_AGENT_VERSION#v} \
    --destination $tmpdir \
    $TOP_DIR/helm/ambassador-agent

package_files=("$tmpdir"/ambassador-agent-*.tgz)
package_file=${package_files[0]}

bucket=${AWS_BUCKET:-datawire-static-files}
prefix=${BUCKET_DIR:-charts-dev}

if [[ -z "$AWS_ACCESS_KEY_ID" ]] ; then
    echo "AWS_ACCESS_KEY_ID is not set"
    exit 1
elif [[ -z "$AWS_SECRET_ACCESS_KEY" ]]; then
    echo "AWS_SECRET_ACCESS_KEY is not set"
    exit 1
fi

echo "Checking that chart hasn't already been pushed by looking in ${bucket} / ${prefix}/${package_file##*/}"
# We don't need the whole object, we just need the metadata
# to see if it exists or not, so this is better than requesting
# the whole tar file.
if aws s3api head-object \
    --bucket "$bucket" \
    --key "${prefix}/${package_file##*/}"
then
    echo "Chart ${prefix}/${package_file##*/} has already been pushed."
    exit 1

fi

# We only push the chart to the S3 bucket. There will be another process
# S3 side that will re-generate the helm chart index when new objects are
# added.
echo "Pushing chart to S3 bucket $bucket"
echo "Pushing ${prefix}/${package_file##*/}"
aws s3api put-object \
    --bucket "$bucket" \
    --key "${prefix}/${package_file##*/}" \
    --body "$package_file"
echo "Successfully pushed ${prefix}/${package_file##*/}"

# Clean up
rm -rf "$tmpdir"