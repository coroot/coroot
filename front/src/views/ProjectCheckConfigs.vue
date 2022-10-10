<template>
    <v-simple-table>
        <thead>
        <tr>
            <th>Inspection</th>
            <th>Project-level override</th>
            <th>Application-level override</th>
        </tr>
        </thead>
        <tbody>
        <template v-for="r in reports">
            <tr v-for="c in r.checks">
                <td>
                    {{ r.name }} / {{ c.name }}
                    <div class="grey--text text-no-wrap">
                        Condition: {{ formatCondition(c.condition, c.global_threshold, c.unit) }}
                    </div>
                </td>
                <td>
                    <template v-if="r.name === 'SLO'">
                        &mdash;
                    </template>
                    <a v-else>
                        <template v-if="c.project_threshold === null">
                            <v-icon small>mdi-file-replace-outline</v-icon>
                        </template>
                        <template v-else>
                            {{ format(c.project_threshold, c.unit) }}
                        </template>
                    </a>
                </td>
                <td>
                    <div v-for="a in c.application_overrides" class="text-no-wrap">
                        {{$api.appId(a.id).name}}:
                        <router-link :to="{name: 'application', params: {id: a.id, report: r.name}}">
                            {{ format(a.threshold, c.unit, a.details) }}
                        </router-link>
                    </div>
                </td>
            </tr>
        </template>
        </tbody>
    </v-simple-table>
</template>

<script>
export default {
    props: {
        projectId: String,
    },

    data() {
        return {
            reports: [],
            loading: false,
            error: '',
            message: '',
        };
    },

    mounted() {
        this.get();
    },

    watch: {
        projectId() {
            this.get();
        }
    },

    methods: {
        formatCondition(condition, global_threshold, unit) {
            return condition.replace('<bucket>', '100ms').replace('<threshold>', this.format(global_threshold, unit));
        },
        format(threshold, unit, details) {
            if (threshold === null) {
                return '—';
            }
            let res = threshold;
            switch (unit) {
                case 'percent':
                    res = threshold + '%';
                    break;
                case 'second':
                    res = this.$moment.duration(threshold, 's').format('s[s] S[ms]', {trim: 'all'});
                    break
            }
            if (details) {
                res += ' ' + details;
            }
            return res;
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.getCheckConfigs((data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.reports = data.reports;
            });
        },
    },
}
</script>

<style scoped>
</style>