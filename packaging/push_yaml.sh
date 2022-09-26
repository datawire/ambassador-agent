#!/bin/bash

set -ex

# This will be running in automation so we don't want the
# interactive pager to pop up
export AWS_PAGER=""

# Get the toplevel dir of the repo so we can run this command
# no matter which directory we are in.
TOP_DIR="$( git rev-parse --show-toplevel)"
echo "$TOP_DIR"

tmpdir=$(mktemp -d)

helm package \
    --version="$A8R_AGENT_VERSION" \
    --app-version="$A8R_AGENT_VERSION" \
    --destination "$tmpdir" \
    "$TOP_DIR/helm/ambassador-agent"
package_files=("$tmpdir"/ambassador-agent-*.tgz)
package_file=${package_files[0]}

yaml_file="${tmpdir}/ambassador-agent.yaml"
helm template --skip-tests ambassador-agent "${package_file}" > "$yaml_file"



bucket=${AWS_BUCKET:-datawire-static-files}
prefix="${BUCKET_DIR:-yaml}/ambassador-agent/${A8R_AGENT_VERSION}"


if [[ -z "$AWS_ACCESS_KEY_ID" ]] ; then
    echo "AWS_ACCESS_KEY_ID is not set"
    exit 1
elif [[ -z "$AWS_SECRET_ACCESS_KEY" ]]; then
    echo "AWS_SECRET_ACCESS_KEY is not set"
    exit 1
fi


echo "Checking that yaml hasn't already been pushed by looking in ${bucket} / ${prefix}/${yaml_file##*/}"
# We don't need the whole object, we just need the metadata
# to see if it exists or not, so this is better than requesting
# the whole tar file.
if aws s3api head-object \
    --bucket "$bucket" \
    --key "${prefix}/${yaml_file##*/}"
then
    echo "Chart ${prefix}/${yaml_file##*/} has already been pushed."
    exit 1

fi

# We only push the chart to the S3 bucket. There will be another process
# S3 side that will re-generate the helm chart index when new objects are
# added.
echo "Pushing chart to S3 bucket $bucket"
echo "Pushing ${prefix}/${yaml_file##*/}"
aws s3api put-object \
    --bucket "$bucket" \
    --key "${prefix}/${yaml_file##*/}" \
    --body "$yaml_file"
echo "Successfully pushed ${prefix}/${yaml_file##*/}"

if [[ "$A8R_AGENT_VERSION" != "*-*" ]]; then
    echo "${A8R_AGENT_VERSION}" > "$tmpdir/stable.txt"
    aws s3api put-object \
        --bucket "$bucket" \
        --key "$BUCKET_DIR/ambassador-agent/stable.txt" \
        --body "$tmpdir/stable.txt"
fi

# Clean up
rm -rf "$tmpdir"
