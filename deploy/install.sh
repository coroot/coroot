#!/bin/sh
set -e

GITHUB_URL="https://github.com/coroot/coroot/releases"
DOWNLOADER=
SUDO=sudo
if [ $(id -u) -eq 0 ]; then
    SUDO=
fi

BIN_DIR=/usr/bin
SYSTEMD_DIR=/etc/systemd/system
VERSION=
SYSTEM_NAME=coroot
SYSTEMD_SERVICE=${SYSTEM_NAME}.service
UNINSTALL_SH=${BIN_DIR}/${SYSTEM_NAME}-uninstall.sh
FILE_SERVICE=${SYSTEMD_DIR}/${SYSTEMD_SERVICE}
FILE_ENV=${SYSTEMD_DIR}/${SYSTEMD_SERVICE}.env
ENV_VARS="^(LISTEN|URL_BASE_PATH|CACHE_TTL|CACHE_GC_INTERVAL|PG_CONNECTION_STRING|DISABLE_USAGE_STATISTICS|READ_ONLY|BOOTSTRAP_PROMETHEUS_URL|BOOTSTRAP_REFRESH_INTERVAL|BOOTSTRAP_PROMETHEUS_EXTRA_SELECTOR|DO_NOT_CHECK_SLO|DO_NOT_CHECK_FOR_DEPLOYMENTS|DO_NOT_CHECK_FOR_UPDATES|BOOTSTRAP_CLICKHOUSE_ADDRESS|BOOTSTRAP_CLICKHOUSE_USER|BOOTSTRAP_CLICKHOUSE_PASSWORD|BOOTSTRAP_CLICKHOUSE_DATABASE)"

info()
{
    echo '[INFO] ' "$@"
}

fatal()
{
    echo '[ERROR] ' "$@" >&2
    exit 1
}

verify_system() {
    if [ -x /bin/systemctl ] || type systemctl > /dev/null 2>&1; then
        return
    fi
    fatal 'Cannot find systemd'
}

verify_executable() {
    if [ ! -x ${BIN_DIR}/coroot ]; then
        fatal "Executable coroot binary not found at ${BIN_DIR}/coroot"
    fi
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
    TMP_BIN=${TMP_DIR}/coroot
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
    URL="${GITHUB_URL}/download/${VERSION}/coroot-${ARCH}"
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
    info "Installing coroot to ${BIN_DIR}/coroot"
    $SUDO chown root:root ${TMP_BIN}
    $SUDO mv -f ${TMP_BIN} ${BIN_DIR}/coroot
}

download() {
    verify_arch
    verify_downloader curl || verify_downloader wget || fatal 'Can not find curl or wget for downloading files'
    setup_tmp
    get_release_version
    download_binary
    setup_binary
}


create_uninstall() {
    info "Creating uninstall script ${UNINSTALL_SH}"
    $SUDO tee ${UNINSTALL_SH} >/dev/null << EOF
#!/bin/sh
set -x
[ \$(id -u) -eq 0 ] || exec sudo \$0 \$@

systemctl stop ${SYSTEM_NAME}
systemctl disable ${SYSTEM_NAME}
systemctl reset-failed ${SYSTEM_NAME}
systemctl daemon-reload

rm -f ${FILE_SERVICE}
rm -f ${FILE_ENV}

remove_uninstall() {
    rm -f ${UNINSTALL_SH}
}
trap remove_uninstall EXIT

rm -rf /var/lib/coroot || true
rm -f ${BIN_DIR}/coroot
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
    sh -c export | while read x v; do echo $v; done | grep -E ${ENV_VARS} | $SUDO tee ${FILE_ENV} >/dev/null
}

create_systemd_service_file() {
    info "systemd: Creating service file ${FILE_SERVICE}"
    $SUDO tee ${FILE_SERVICE} >/dev/null << EOF
[Unit]
Description=Coroot
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
ExecStart=${BIN_DIR}/coroot --data-dir=/var/lib/coroot
EOF
}

create_service_file() {
    create_systemd_service_file
    return 0
}

get_installed_hashes() {
    $SUDO sha256sum ${BIN_DIR}/coroot ${FILE_SERVICE} ${FILE_ENV} 2>&1 || true
}

systemd_enable() {
    info "systemd: Enabling ${SYSTEM_NAME} unit"
    $SUDO systemctl enable ${FILE_SERVICE} >/dev/null
    $SUDO systemctl daemon-reload >/dev/null
}

systemd_start() {
    info "systemd: Starting ${SYSTEM_NAME}"
    $SUDO systemctl restart ${SYSTEM_NAME}
}


service_enable_and_start() {
    systemd_enable

    POST_INSTALL_HASHES=$(get_installed_hashes)
    if [ "${PRE_INSTALL_HASHES}" = "${POST_INSTALL_HASHES}" ]; then
        info 'No change detected so skipping service start'
        return
    fi

    systemd_start

    return 0
}

{
    verify_system
    download
    create_uninstall
    systemd_disable
    create_env_file
    create_service_file
    service_enable_and_start
}
