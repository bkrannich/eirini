#!/bin/bash

set -euo pipefail

IFS=$'\n\t'

USAGE=$(
  cat <<EOF
Usage: patch-me-if-you-can.sh [options] [ <component-name> ... ]
Options:
  -c <cluster-name>  - required unless skipping deloyment
  -s  skip docker builds
  -S  skip deployment (only update the eirini release SHAs)
  -d  deploy the helmless YAML, rather than helm release
  -o <additional-values.yml>  - use additional values from file
  -h  this help
EOF
)
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
readonly EIRINI_BASEDIR=$(realpath "$SCRIPT_DIR/..")
readonly EIRINI_RELEASE_BASEDIR=$(realpath "$SCRIPT_DIR/../../eirini-release")
readonly EIRINI_PRIVATE_CONFIG_BASEDIR=$(realpath "$SCRIPT_DIR/../../eirini-private-config")
readonly EIRINI_CI_BASEDIR="$HOME/workspace/eirini-ci"
readonly CF4K8S_DIR="$HOME/workspace/cf-for-k8s"
readonly CAPI_DIR="$HOME/workspace/capi-release"
readonly CAPIK8S_DIR="$HOME/workspace/capi-k8s-release"
readonly PATCH_TAG='patch-me-if-you-can'

main() {
  if [ "$#" == "0" ]; then
    echo "$USAGE"
    exit 1
  fi

  local cluster_name="" additional_values skip_docker_build="false" use_helmless="false" skip_deployment="false"

  additional_values=""
  while getopts "hc:o:sSd" opt; do
    case ${opt} in
      h)
        echo "$USAGE"
        exit 0
        ;;
      c)
        cluster_name=$OPTARG
        ;;
      s)
        skip_docker_build="true"
        ;;
      S)
        skip_deployment="true"
        ;;
      d)
        use_helmless="true"
        ;;
      o)
        additional_values=$OPTARG
        if ! [[ -f $additional_values ]]; then
          echo "Provided values file does not exist: $additional_values"
          echo $USAGE
          exit 1
        fi
        ;;
      \?)
        echo "Invalid option: $OPTARG" 1>&2
        echo "$USAGE"
        ;;
      :)
        echo "Invalid option: $OPTARG requires an argument" 1>&2
        echo "$USAGE"
        ;;
    esac
  done
  shift $((OPTIND - 1))

  if [[ "$skip_deployment" == "false" && -z "$cluster_name" ]]; then
    echo "Cluster name not provided"
    echo "$USAGE"
    exit 1
  fi

  if [[ "$(current_cluster_name)" =~ "gke_.*${cluster_name}\$" ]]; then
    echo "Your current cluster is $(current_cluster_name), but you want to update $cluster_name. Please target $cluster_name"
    echo "gcloudcluster $cluster_name"
    exit 1
  fi

  if [[ "$use_helmless" != "true" ]]; then
    echo "Checking out latest stable cf-for-k8s..."
    checkout_stable_cf4k8s
  fi

  if [ "$skip_docker_build" != "true" ]; then
    if [ "$#" == "0" ]; then
      echo "No components specified. Nothing to do."
      echo "If you want to helm upgrade without building containers, please pass the '-s' flag"
      exit 0
    fi
    local component
    for component in "$@"; do
      if is_cloud_controller $component; then
        checkout_stable_cf_for_k8s_deps
        build_ccng_image
      else
        update_component "$component"
      fi
    done
  fi

  if [[ "$skip_deployment" == "true" ]]; then
    exit 0
  fi

  if [[ "$use_helmless" == "true" ]]; then
    deploy_helmless
    exit 0
  fi

  pull_private_config
  patch_cf_for_k8s "$additional_values"
  deploy_cf "$cluster_name"
}

is_cloud_controller() {
  local component
  component="$1"
  [[ "$component" =~ cloud.controller ]] || [[ "$component" =~ "ccng" ]] || [[ "$component" =~ "capi" ]] || [[ "$component" =~ "cc" ]]
}

checkout_stable_cf4k8s() {
  pushd "$CF4K8S_DIR"
  {
    echo "Cleaning dirty state in cf-for-k8s..."
    git checkout . && git clean -ffd
    echo "Checking out latest release..."
    git fetch --tags
    stable_cf4k8s_release="$(git tag | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$" | sort --reverse | head -1)"
    git checkout "$stable_cf4k8s_release"
    echo "cf-for-k8s version: $stable_cf4k8s_release"
  }
  popd
}

checkout_stable_cf_for_k8s_deps() {
  local stable_cf4k8s_release capi_k8s_release_sha capi_release_sha

  echo "Dear future us, take a deep breath as I am checking out stable revisions of cf-for-k8s dependencies for you..."

  echo "Getting the vendored revision of capi-k8s-release..."

  pushd "$CF4K8S_DIR"
  {
    capi_k8s_release_sha=$(yq r vendir.lock.yml 'directories.(path==config/capi/_ytt_lib/capi-k8s-release).contents[0].git.sha')
    echo "capi-k8s-release version: $capi_k8s_release_sha"
  }
  popd

  pushd "$CAPIK8S_DIR"
  {
    echo "Getting the revision of the cloud_controller_ng submodule"
    git checkout "$capi_k8s_release_sha"
    ccng_sha="$(git show $capi_k8s_release_sha | grep cloud_controller_ng/commit | awk -F '/' '{print $NF}')"
    echo "cloud_controller_ng version: $ccng_sha"
  }
  popd

  pushd "$CAPI_DIR/src/cloud_controller_ng"
  {
    git stash
    git checkout "$ccng_sha"
    git stash pop
  }
  popd

  echo "All done!"
}

build_ccng_image() {
  export IMAGE_DESTINATION_CCNG="docker.io/eirini/dev-ccng"
  export IMAGE_DESTINATION_CF_API_CONTROLLERS="docker.io/eirini/dev-controllers"
  export IMAGE_DESTINATION_REGISTRY_BUDDY="docker.io/eirini/dev-registry-buddy"
  export IMAGE_DESTINATION_BACKUP_METADATA="docker.io/eirini/dev-backup-metadata"
  git -C "$CAPIK8S_DIR" checkout values/images.yml
  "$CAPIK8S_DIR"/scripts/build-into-values.sh "$CAPIK8S_DIR/values/images.yml"
  "$CAPIK8S_DIR"/scripts/bump-cf-for-k8s.sh
}

update_component() {
  local component
  component=$1

  echo "--- Patching component $component ---"
  docker_build "$component"
  docker_push "$component"
  update_image_in_yaml_files "$component" "$EIRINI_RELEASE_BASEDIR/helm/eirini/templates"
  update_image_in_yaml_files "$component" "$EIRINI_RELEASE_BASEDIR/deploy"
}

docker_build() {
  echo "Building docker image for $1"
  pushd "$EIRINI_BASEDIR"
  make --directory=docker "$component" TAG="$PATCH_TAG"
  popd
}

docker_push() {
  echo "Pushing docker image for $1"
  pushd "$EIRINI_BASEDIR"
  make --directory=docker push-$component TAG="$PATCH_TAG"
  popd
}

update_image_in_yaml_files() {
  deploy_dir=$2
  echo "Applying docker image of $1 to kubernetes cluster in $2"

  pushd "$deploy_dir"
  {
    local file new_image_ref
    file=$(rg -l "image: eirini/${1}")
    new_image_ref="$(docker inspect --format='{{index .RepoDigests 0}}' "eirini/${1}:$PATCH_TAG")"
    sed -i -e "s|image: eirini/${1}.*$|image: ${new_image_ref}|g" "$file"
  }
  popd
}

patch_cf_for_k8s() {
  local build_path eirini_values user_values
  user_values="$1"
  rm -rf "$CF4K8S_DIR/build/eirini/_vendir/eirini"

  build_path="$CF4K8S_DIR/build/eirini/"
  eirini_values="$build_path/eirini-values.yml"

  if ! [[ -z "$user_values" ]]; then
    yq merge --inplace "$eirini_values" "$user_values"
  fi

  cp -r "$EIRINI_RELEASE_BASEDIR/helm/eirini" "$CF4K8S_DIR/build/eirini/_vendir/"

  "$CF4K8S_DIR"/build/eirini/build.sh
}

deploy_helmless() {
  "$EIRINI_RELEASE_BASEDIR/deploy/scripts/deploy.sh"
}

deploy_cf() {
  local cluster_name
  cluster_name="$1"
  shift 1
  kapp deploy -a cf -f <(
    ytt -f "$CF4K8S_DIR/config" \
      -f "$EIRINI_CI_BASEDIR/cf-for-k8s" \
      -f "$EIRINI_PRIVATE_CONFIG_BASEDIR/environments/kube-clusters/"${cluster_name}"/default-values.yml" \
      -f "$EIRINI_PRIVATE_CONFIG_BASEDIR/environments/kube-clusters/"${cluster_name}"/loadbalancer-values.yml" \
      $@
  ) -y
}

pull_private_config() {
  pushd "$EIRINI_PRIVATE_CONFIG_BASEDIR"
  git pull --rebase
  popd
}

current_cluster_name() {
  kubectl config current-context | cut -d / -f 1
}

main "$@"
