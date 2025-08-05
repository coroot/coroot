<template>
    <div>
        <v-progress-linear v-if="loading" indeterminate height="2" color="success" style="position: absolute; top: 0; left: 0" />

        <v-form v-if="form" v-model="valid" ref="form" style="max-width: 800px">
            <v-alert v-if="form.global" color="primary" outlined text>
                This project uses a global ClickHouse configuration that can't be changed through the UI
            </v-alert>

            <div class="subtitle-1">Protocol</div>
            <v-radio-group v-model="form.protocol" row dense class="mt-0" :disabled="form.global">
                <v-radio label="Native" value="native"></v-radio>
                <v-radio label="HTTP" value="http"></v-radio>
            </v-radio-group>

            <div class="subtitle-1">Clickhouse address</div>
            <div class="caption"></div>
            <v-text-field
                outlined
                dense
                v-model="form.addr"
                :rules="[$validators.isAddr]"
                placeholder="clickhouse:9000"
                hide-details="auto"
                class="flex-grow-1"
                clearable
                single-line
                :disabled="form.global"
            />

            <div class="subtitle-1 mt-3">Credentials</div>
            <div class="d-flex gap">
                <v-text-field
                    v-model="form.auth.user"
                    :rules="[$validators.notEmpty]"
                    label="username"
                    outlined
                    dense
                    hide-details
                    single-line
                    :disabled="form.global"
                />
                <v-text-field
                    v-model="form.auth.password"
                    label="password"
                    type="password"
                    outlined
                    dense
                    hide-details
                    single-line
                    :disabled="form.global"
                />
            </div>

            <div class="subtitle-1 mt-3">Database</div>
            <v-text-field v-model="form.database" :rules="[$validators.notEmpty]" outlined dense hide-details single-line :disabled="form.global" />

            <v-checkbox v-model="form.tls_enable" label="Enable TLS" hide-details class="my-3" :disabled="form.global" />
            <v-checkbox
                v-model="form.tls_skip_verify"
                :disabled="!form.tls_enable || form.global"
                label="Skip TLS verify"
                hide-details
                class="my-2"
            />

            <div v-if="form.addr" class="mt-4">
                <div class="subtitle-1">Storage Usage</div>
                <div class="caption">Includes all replicas and shards</div>
                <v-simple-table v-if="tableSizes && tableSizes.length > 0" dense class="table mt-2">
                    <thead>
                        <tr>
                            <th>Data type</th>
                            <th>Size</th>
                            <th>Compression</th>
                            <th>TTL</th>
                            <th>Data since</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="item in tableSizes" :key="item.table">
                            <td>{{ item.table }}</td>
                            <td>{{ formatBytes(item.bytes_on_disk) }}</td>
                            <td>{{ item.compression_ratio.toFixed(1) }}x</td>
                            <td>
                                <div v-if="item.ttl_seconds">
                                    {{ $format.durationPretty(item.ttl_seconds * 1000) }}
                                </div>
                                <div v-else>-</div>
                            </td>
                            <td>
                                <div v-if="item.data_since">{{ $format.timeSinceNow(new Date(item.data_since).getTime()) }} ago</div>
                                <div v-else>-</div>
                            </td>
                        </tr>
                    </tbody>
                </v-simple-table>
                <div v-else class="pa-4 text-center caption grey--text mt-2">No table information available</div>
            </div>

            <div v-if="form.addr && topology && topology.length > 0" class="mt-4">
                <div class="subtitle-1">Cluster Topology</div>
                <v-simple-table dense class="table mt-2">
                    <thead>
                        <tr>
                            <th>Server</th>
                            <th>Disk</th>
                            <th>Free Space</th>
                            <th>Total Space</th>
                            <th>Usage</th>
                        </tr>
                    </thead>
                    <tbody>
                        <template v-for="node in topology">
                            <template v-if="getServerDisks(node.host_name + ':' + node.port).length === 0">
                                <tr :key="node.host_name + ':' + node.port + ':no-disks'">
                                    <td class="server-name">
                                        <div :title="node.host_name + ':' + node.port">
                                            {{ truncateServerName(node.host_name + ':' + node.port) }}
                                        </div>
                                        <div class="caption grey--text">shard: {{ node.shard_num }}, replica: {{ node.replica_num }}</div>
                                    </td>
                                    <td colspan="4" class="caption grey--text">No disk information available</td>
                                </tr>
                            </template>
                            <template v-else>
                                <tr
                                    v-for="(disk, index) in getServerDisks(node.host_name + ':' + node.port)"
                                    :key="node.host_name + ':' + node.port + ':' + disk.name"
                                >
                                    <td v-if="index === 0" :rowspan="getServerDisks(node.host_name + ':' + node.port).length" class="server-name">
                                        <div :title="node.host_name + ':' + node.port">
                                            {{ truncateServerName(node.host_name + ':' + node.port) }}
                                        </div>
                                        <div class="caption grey--text">shard: {{ node.shard_num }}, replica: {{ node.replica_num }}</div>
                                    </td>
                                    <td>
                                        <div>{{ disk.path }}</div>
                                        <div class="caption grey--text">type: {{ disk.type }}, name: {{ disk.name }}</div>
                                    </td>
                                    <td>{{ formatBytes(disk.free_space) }}</td>
                                    <td>{{ formatBytes(disk.total_space) }}</td>
                                    <td>
                                        <v-progress-linear
                                            background-color="blue lighten-3"
                                            height="16"
                                            color="blue lighten-1"
                                            :value="getDiskUsagePercent(disk)"
                                            style="min-width: 64px"
                                        >
                                            <span style="font-size: 14px">{{ getDiskUsagePercent(disk) }}%</span>
                                        </v-progress-linear>
                                    </td>
                                </tr>
                            </template>
                        </template>
                    </tbody>
                </v-simple-table>
            </div>

            <v-alert v-if="error" color="red" icon="mdi-alert-octagon-outline" outlined text>
                {{ error }}
            </v-alert>
            <v-alert v-if="message" color="green" outlined text>
                {{ message }}
            </v-alert>
            <div class="mt-3">
                <v-btn v-if="saved.addr && !form.addr" block color="error" @click="del" :loading="loading">Delete</v-btn>
                <v-btn v-else block color="primary" @click="save" :disabled="!form.addr || !valid || form.global" :loading="loading"
                    >Test & Save</v-btn
                >
            </div>
        </v-form>
    </div>
</template>

<script>
import { formatBytes } from '@/utils/format';

export default {
    data() {
        return {
            form: null,
            valid: false,
            loading: false,
            error: '',
            message: '',
            saved: null,
            topology: null,
            tableSizes: null,
            serverDisks: null,
        };
    },

    mounted() {
        this.get();
    },

    computed: {
        changed() {
            return JSON.stringify(this.form) !== JSON.stringify(this.saved);
        },
    },

    methods: {
        formatBytes,
        getDiskUsagePercent(disk) {
            if (disk.total_space === 0) return 0;
            const usedSpace = disk.total_space - disk.free_space;
            return Math.round((usedSpace / disk.total_space) * 100);
        },
        getServerDisks(addr) {
            if (!this.serverDisks) return [];
            const server = this.serverDisks.find((s) => s.addr === addr);
            return server ? server.disks || [] : [];
        },
        truncateServerName(serverName) {
            if (serverName.length <= 30) {
                return serverName;
            }
            const firstDotIndex = serverName.indexOf('.');
            if (firstDotIndex > 0 && firstDotIndex <= 30) {
                return serverName.substring(0, firstDotIndex) + '...';
            }
            return serverName.substring(0, 30) + '...';
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getIntegrations('clickhouse', (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                if (data.form) {
                    this.form = data.form;
                    this.topology = data.topology;
                    this.tableSizes = data.table_sizes;
                    this.serverDisks = data.server_disks;
                } else {
                    this.form = data;
                    this.topology = null;
                    this.tableSizes = null;
                    this.serverDisks = null;
                }
                this.saved = JSON.parse(JSON.stringify(this.form));
            });
        },
        save() {
            this.loading = true;
            this.error = '';
            this.message = '';
            const form = JSON.parse(JSON.stringify(this.form));
            this.$api.saveIntegrations('clickhouse', 'save', form, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.$events.emit('refresh');
                this.message = 'Settings were successfully updated.';
                setTimeout(() => {
                    this.message = '';
                }, 1000);
                this.get();
            });
        },
        del() {
            this.saving = true;
            this.error = '';
            this.message = '';
            this.$api.saveIntegrations('clickhouse', 'del', null, (data, error) => {
                this.saving = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.get();
            });
        },
    },
};
</script>

<style scoped>
.gap {
    gap: 16px;
}

.server-name {
    max-width: 30ch;
}

.table:deep(tr:hover) {
    background-color: unset !important;
}

.table:deep(th),
.table:deep(td) {
    padding: 4px 8px !important;
}

.table:deep(th) {
    white-space: nowrap;
}
</style>
