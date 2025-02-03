#!/bin/sh
set -e

SUDO=sudo
if [ $(id -u) -eq 0 ]; then
    SUDO=
fi

BIN_DIR=/usr/bin
SYSTEMD_DIR=/etc/systemd/system
DATA_DIR=/var/lib/coroot

DOWNLOADER=
GITHUB_URL=
VERSION=
SYSTEM_NAME=
SYSTEM_DESCRIPTION=
FILE_SERVICE=
FILE_ENV=
ARGS=

info() {
    echo '[INFO] ' "$@"
}

fatal() {
    echo '[ERROR] ' "$@" >&2
    exit 1
}

verify_system() {
    if [ -x /bin/systemctl ] || type systemctl > /dev/null 2>&1; then
        return
    fi
    fatal 'Cannot find systemd'
}

verify_arch() {
    if [ -z "$ARCH" ]; then
        ARCH=$(uname -m)
    fi
    case $ARCH in
        amd64)
            ARCH=amd64
            ;;
        x86_64)
            ARCH=amd64
            ;;
        arm64)
            ARCH=arm64
            ;;
        aarch64)
            ARCH=arm64
            ;;
        *)
            fatal "Unsupported architecture $ARCH"
    esac
}

verify_downloader() {
    [ -x "$(command -v $1)" ] || return 1
    DOWNLOADER=$1
    return 0
}

setup_tmp() {
    TMP_DIR=$(mktemp -d -t coroot-install.XXXXXXXXXX)
    TMP_BIN=${TMP_DIR}/${SYSTEM_NAME}
    cleanup() {
        code=$?
        set +e
        trap - EXIT
        rm -rf ${TMP_DIR}
        exit $code
    }
    trap cleanup INT EXIT
}

get_release_version() {
    info "Finding the latest release"
    latest_release_url=${GITHUB_URL}/latest
    case $DOWNLOADER in
        curl)
            VERSION=$(curl -w '%{url_effective}' -L -s -S ${latest_release_url} -o /dev/null | sed -e 's|.*/||')
            ;;
        wget)
            VERSION=$(wget -SqO /dev/null ${latest_release_url} 2>&1 | grep -i Location | sed -e 's|.*/||')
            ;;
        *)
            fatal "Incorrect downloader executable '$DOWNLOADER'"
            ;;
    esac
    info "The latest release is ${VERSION}"
}

download_binary() {
    info "Downloading binary"
    URL="${GITHUB_URL}/download/${VERSION}/${SYSTEM_NAME}-${ARCH}"
    set +e
    case $DOWNLOADER in
        curl)
            curl -o ${TMP_BIN} -sfL ${URL}
            ;;
        wget)
            wget -qO ${TMP_BIN} ${URL}
            ;;
        *)
            fatal "Incorrect executable '$DOWNLOADER'"
            ;;
    esac

    [ $? -eq 0 ] || fatal 'Download failed'
    set -e
}

setup_binary() {
    chmod 755 ${TMP_BIN}
    info "Installing to ${BIN_DIR}/${SYSTEM_NAME}"
    $SUDO chown root:root ${TMP_BIN}
    $SUDO mv -f ${TMP_BIN} ${BIN_DIR}/${SYSTEM_NAME}
}

download() {
    setup_tmp
    get_release_version
    download_binary
    setup_binary
}

create_uninstall() {
    UNINSTALL_SH=${BIN_DIR}/coroot-uninstall.sh
    info "Creating uninstall script ${UNINSTALL_SH}"
    $SUDO tee ${UNINSTALL_SH} >/dev/null << EOF
#!/bin/sh
set -x
[ \$(id -u) -eq 0 ] || exec sudo \$0 \$@

systemctl stop coroot
systemctl disable coroot
systemctl reset-failed coroot

systemctl stop coroot-cluster-agent
systemctl disable coroot-cluster-agent
systemctl reset-failed coroot-cluster-agent

systemctl daemon-reload

rm -f ${SYSTEMD_DIR}/coroot.service
rm -f ${SYSTEMD_DIR}/coroot.service.env
rm -f ${SYSTEMD_DIR}/coroot-cluster-agent.service
rm -f ${SYSTEMD_DIR}/coroot-cluster-agent.service.env

remove_uninstall() {
    rm -f ${UNINSTALL_SH}
}
trap remove_uninstall EXIT

rm -rf ${DATA_DIR} || true
rm -f ${BIN_DIR}/coroot
rm -f ${BIN_DIR}/coroot-cluster-agent
EOF
    $SUDO chmod 755 ${UNINSTALL_SH}
    $SUDO chown root:root ${UNINSTALL_SH}
}

systemd_disable() {
    $SUDO systemctl disable ${SYSTEM_NAME} >/dev/null 2>&1 || true
    $SUDO rm -f ${FILE_SERVICE} || true
    $SUDO rm -f ${FILE_ENV} || true
}

create_env_file() {
    info "env: Creating environment file ${FILE_ENV}"
    $SUDO touch ${FILE_ENV}
    $SUDO chmod 0600 ${FILE_ENV}
    case $SYSTEM_NAME in
        coroot)
            env_vars="LISTEN|URL_BASE_PATH|CACHE_TTL|CACHE_GC_INTERVAL|PG_CONNECTION_STRING|DISABLE_USAGE_STATISTICS|READ_ONLY|BOOTSTRAP_PROMETHEUS_URL|BOOTSTRAP_REFRESH_INTERVAL|BOOTSTRAP_PROMETHEUS_EXTRA_SELECTOR|DO_NOT_CHECK_SLO|DO_NOT_CHECK_FOR_DEPLOYMENTS|DO_NOT_CHECK_FOR_UPDATES|BOOTSTRAP_CLICKHOUSE_ADDRESS|BOOTSTRAP_CLICKHOUSE_USER|BOOTSTRAP_CLICKHOUSE_PASSWORD|BOOTSTRAP_CLICKHOUSE_DATABASE"
            sh -c export | while read x v; do echo $v; done | grep -E "^(${env_vars})" | $SUDO tee ${FILE_ENV} >/dev/null
            ;;
        coroot-cluster-agent)
            host=$(sh -c export | sed -nr "s/.*LISTEN='(.+):.*'/\1/p")
            if [ -z $host ]; then
                host=127.0.0.1
            fi
            port=$(sh -c export | sed -nr "s/.*LISTEN='.*:([0-9]+)'/\1/p")
            if [ -z $port ]; then
                port=8080
            fi
            interval=$(sh -c export | sed -nr "s/.*BOOTSTRAP_REFRESH_INTERVAL='(.+)'/\1/p")
            if [ -z $interval ]; then
                interval=15s
            fi
            $SUDO sh -c "echo \"COROOT_URL='http://${host}:${port}'\" >> ${FILE_ENV}"
            $SUDO sh -c "echo \"METRICS_SCRAPE_INTERVAL='${interval}'\" >> ${FILE_ENV}"
            ;;
    esac
}

create_service_file() {
    info "systemd: Creating service file ${FILE_SERVICE}"
    $SUDO tee ${FILE_SERVICE} >/dev/null << EOF
[Unit]
Description=${SYSTEM_DESCRIPTION}
Documentation=https://coroot.com
Wants=network-online.target
After=network-online.target

[Install]
WantedBy=multi-user.target

[Service]
Type=exec
EnvironmentFile=-/etc/default/%N
EnvironmentFile=-/etc/sysconfig/%N
EnvironmentFile=-${FILE_ENV}
KillMode=process
Delegate=yes
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
TimeoutStartSec=0
Restart=always
RestartSec=5s
ExecStart=${BIN_DIR}/${SYSTEM_NAME} ${ARGS}
EOF
}

service_enable_and_start() {
    info "systemd: Enabling ${SYSTEM_NAME}"
    $SUDO systemctl enable ${FILE_SERVICE} >/dev/null
    $SUDO systemctl daemon-reload >/dev/null

    info "systemd: Starting ${SYSTEM_NAME}"
    $SUDO systemctl restart ${SYSTEM_NAME}
}

install() {
    SYSTEM_NAME=$1
    SYSTEM_DESCRIPTION=$2
    ARGS=$3

    FILE_SERVICE=${SYSTEMD_DIR}/${SYSTEM_NAME}.service
    FILE_ENV=${FILE_SERVICE}.env
    GITHUB_URL="https://github.com/coroot/${SYSTEM_NAME}/releases"

    echo "*** INSTALLING ${SYSTEM_NAME} ***"
    download
    systemd_disable
    create_env_file
    create_service_file
    service_enable_and_start
}

{
    verify_system
    verify_arch
    verify_downloader curl || verify_downloader wget || fatal 'Can not find curl or wget for downloading files'

    create_uninstall

    install coroot "Coroot" "--data-dir=${DATA_DIR}"
    install coroot-cluster-agent "Coroot Cluster Agent" "--metrics-wal-dir=${DATA_DIR}"
}
