<template>
    <div class="qb" @click="enter" v-click-outside="leave">
        <div v-for="(f, i) in filters" class="filter">
            <v-chip label close close-icon="mdi-close" @click:close="del(i)">
                <div class="nowrap">{{ formatFilter(f) }}</div>
            </v-chip>
        </div>

        <div class="filter">
            <v-chip v-if="filter.name" label close close-icon="mdi-close" @click:close="del()">
                <div class="nowrap">{{ formatFilter(filter) }}</div>
            </v-chip>

            <v-menu
                :value="menu"
                ref="menu"
                :attach="$el"
                tile
                offset-y
                max-height="300"
                disable-keys
                :open-on-click="false"
                :close-on-click="false"
                :close-on-content-click="false"
            >
                <template #activator="{}">
                    <v-text-field v-model="str" ref="input" dense hide-details flat solo class="input" @keydown="keydown" />
                </template>
                <v-progress-linear v-if="loading" indeterminate />
                <v-list v-else dense class="list">
                    <v-list-item v-if="error" dense class="item" @click="get">Failed to load options</v-list-item>
                    <template v-else-if="_items.length">
                        <v-list-item v-for="i in _items" dense class="item" @click="select(i)">
                            {{ i }}
                        </v-list-item>
                    </template>
                    <v-list-item v-else-if="mode === 'value' && str" dense class="item" @click="select(str)">
                        Use custom value: {{ str }}
                    </v-list-item>
                    <v-list-item v-else dense class="item"> No options found </v-list-item>
                </v-list>
            </v-menu>
        </div>
    </div>
</template>

<script>
export default {
    props: {
        value: Array,
        items: Array,
        loading: Boolean,
        error: String,
    },

    data() {
        return {
            filters: this.value,
            filter: { name: '', op: '', value: '' },
            mode: 'name',
            str: '',
            menu: false,
            item: 0,
        };
    },

    computed: {
        _items() {
            return this.items.filter((i) => i.toLocaleLowerCase().includes(this.str.toLocaleLowerCase()));
        },
    },

    watch: {
        menu() {
            this.menu && this.get();
        },
        _items() {
            if (this.item >= this._items.length) {
                this.item = 0;
            }
            requestAnimationFrame(() => {
                this.$refs.menu.getTiles();
                this.$refs.menu.listIndex = this.item;
                const tile = this.$refs.menu.tiles[this.item];
                tile && tile.classList.add('v-list-item--highlighted');
            });
        },
        item() {
            this.$refs.menu.listIndex = this.item;
        },
        filters() {
            this.$nextTick(this.$refs.menu?.updateDimensions);
            this.$emit('input', this.filters);
        },
        filter: {
            handler() {
                this.$nextTick(this.$refs.menu?.updateDimensions);
            },
            deep: true,
        },
    },

    methods: {
        del(i) {
            if (i !== undefined) {
                this.filters.splice(i, 1);
            } else {
                this.mode = 'name';
                this.filter = { name: '', op: '', value: '' };
                this.item = 0;
            }
            this.get();
        },
        select(v) {
            this.str = '';
            this.filter[this.mode] = v;
            switch (this.mode) {
                case 'name':
                    this.mode = 'op';
                    break;
                case 'op':
                    this.mode = 'value';
                    break;
                case 'value':
                    this.filters.push(this.filter);
                    this.mode = 'name';
                    this.filter = { name: '', op: '', value: '' };
                    break;
            }
            this.get();
            this.item = 0;
        },
        keydown(e) {
            if (e.code === 'Escape') {
                this.leave();
                return;
            } else {
                if (!this.menu) {
                    this.menu = true;
                    return;
                }
                if (this.error) {
                    this.get();
                    return;
                }
            }
            const l = this._items.length - 1;
            switch (e.code) {
                case 'ArrowDown':
                    this.item = this.item === l ? 0 : this.item + 1;
                    break;
                case 'ArrowUp':
                    this.item = this.item === 0 ? l : this.item - 1;
                    break;
                case 'Enter':
                    if (this._items[this.item]) {
                        this.select(this._items[this.item]);
                        break;
                    }
                    if (this.mode === 'value' && this.str) {
                        this.select(this.str);
                        break;
                    }
                    break;
            }
        },
        get() {
            this.$emit('get', this.mode, this.filter.name);
            this.enter();
        },
        enter() {
            this.menu = true;
            this.$refs.input?.focus();
        },
        leave() {
            this.menu = false;
            this.filter = { name: '', op: '', value: '' };
            this.str = '';
            this.mode = 'name';
        },
        formatFilter(filter) {
            let op = filter.op;
            if (filter.op === 'contains') {
                op = '🔍';
            }
            return filter.name + ' ' + op + ' ' + filter.value;
        },
    },
};
</script>

<style scoped>
.qb {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 4px 8px;
    min-height: 40px;
    max-width: 100%;
    min-width: 0;
}
.filter {
    display: flex;
    flex-wrap: nowrap;
    gap: 4px;
    max-width: 100%;
}
.filter:deep(.v-chip) {
    height: 30px !important;
}
.input {
    max-width: 200px;
}
.input:deep(.v-input__control) {
    min-height: 28px !important;
}
.input:deep(.v-input__slot) {
    padding: 0 !important;
    background-color: unset !important;
}
.list {
    padding: 0;
}
.item {
    min-height: unset !important;
    padding: 4px 8px !important;
}
</style>
