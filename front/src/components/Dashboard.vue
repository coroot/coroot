<template>
    <div>
        <template v-for="(s, si) in sections">
            <div v-if="s.header" :key="name + ':header:' + s.header" class="section">
                <h2 class="group-header text-h6" @click="toggle(s.header)">
                    <v-icon>{{ collapsed[s.header] ? 'mdi-chevron-right' : 'mdi-chevron-down' }}</v-icon>
                    {{ s.header }}
                </h2>
                <div v-show="!collapsed[s.header]">
                    <div class="d-flex flex-wrap">
                        <Widget v-for="(w, i) in s.widgets" :key="name + ':' + si + ':' + i" :w="w" class="my-5" :style="widgetStyle(w)" />
                    </div>
                </div>
            </div>
            <div v-else :key="name + ':section:' + si" class="d-flex flex-wrap">
                <Widget v-for="(w, i) in s.widgets" :key="name + ':' + si + ':' + i" :w="w" class="my-5" :style="widgetStyle(w)" />
            </div>
        </template>
    </div>
</template>

<script>
import Widget from './Widget';
import { local } from '../utils/storage';

export default {
    props: {
        name: String,
        widgets: Array,
    },

    components: { Widget },

    data() {
        const collapsed = {};
        (local('collapsed-groups:' + this.name) || []).forEach((g) => {
            collapsed[g] = true;
        });
        return { collapsed };
    },

    computed: {
        storageKey() {
            return 'collapsed-groups:' + this.name;
        },
        sections() {
            const res = [];
            let current = null;
            (this.widgets || []).forEach((w) => {
                if (w.group_header) {
                    current = { header: w.group_header, widgets: [] };
                    res.push(current);
                    return;
                }
                if (current && w.group === current.header) {
                    current.widgets.push(w);
                    return;
                }
                current = null;
                if (!res.length || res[res.length - 1].header !== null) {
                    res.push({ header: null, widgets: [] });
                }
                res[res.length - 1].widgets.push(w);
            });
            return res.filter((s) => s.header || s.widgets.length);
        },
    },

    methods: {
        widgetStyle(w) {
            return { width: this.$vuetify.breakpoint.mdAndUp ? w.width || '50%' : '100%' };
        },
        toggle(header) {
            this.$set(this.collapsed, header, !this.collapsed[header]);
            local(
                this.storageKey,
                Object.keys(this.collapsed).filter((g) => this.collapsed[g]),
            );
        },
    },
};
</script>

<style scoped>
.section {
    width: 100%;
}
.group-header {
    padding: 4px 8px;
    background-color: var(--background-color-hi);
    border-radius: 4px;
    cursor: pointer;
    user-select: none;
}
</style>
