set -ex

pk=$(base64 -w 0 /workspace/certs/pk.cer)
kek=$(base64 -w 0 /workspace/certs/kek.cer)
db=$(base64 -w 0 /workspace/certs/db.cer)

/workspace/daisy -print_perf \
  -zone=${ZONE} \
  -var:publish_project=${PROJECT_ID} \
  -var:gcs_root=${GCS_ROOT} \
  -var:image_output_bucket=${IMAGE_OUTPUT_BUCKET} \
  -var:kernel_package=${KERNEL_PACKAGE} \
  -var:packages="${PACKAGES}" \
  -var:pk=${pk} \
  -var:kek=${kek} \
  -var:db=${db} \
  build/build-image.wf.json