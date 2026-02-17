<template>
    <div>
        <v-simple-table>
            <thead>
                <tr>
                    <th>Category</th>
                    <th>Patterns</th>
                    <th>Notify of incidents</th>
                    <th>Notify of deployments</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                <tr v-for="c in categories">
                    <td class="text-no-wrap">
                        <div class="text-no-wrap">{{ c.name }}</div>
                    </td>
                    <td style="line-height: 2em">
                        <div v-if="c.default" class="grey--text">
                            The default category containing applications that don't fit into other categories
                        </div>
                        <template v-else v-for="p in (c.builtin_patterns + ' ' + c.custom_patterns).split(' ').filter((p) => !!p)">
                            <span class="pattern">{{ p }}</span>
                            &nbsp;
                        </template>
                    </td>
                    <td>
                        {{ c.notification_settings.incidents.enabled ? 'on' : 'off' }}
                    </td>
                    <td>
                        {{ c.notification_settings.deployments.enabled ? 'on' : 'off' }}
                    </td>
                    <td>
                        <div class="d-flex">
                            <v-btn icon small @click="open(c.name, 'edit')"><v-icon small>mdi-pencil</v-icon></v-btn>
                            <v-btn v-if="!c.builtin" icon small @click="open(c.name, 'delete')"><v-icon small>mdi-trash-can-outline</v-icon></v-btn>
                        </div>
                    </td>
                </tr>
            </tbody>
        </v-simple-table>

        <v-btn color="primary" class="mt-2" @click="open('', 'add')" small>Add a category</v-btn>

        <ApplicationCategoryForm v-if="action" v-model="action" :name="category.name" :extra_custom_patterns="category.extra_custom_patterns" />
    </div>
</template>

<script>
import ApplicationCategoryForm from '@/views/ApplicationCategoryForm.vue';

export default {
    props: {
        projectId: String,
    },

    components: { ApplicationCategoryForm },

    data() {
        return {
            loading: false,
            error: '',
            message: '',
            categories: [],
            action: '',
            category: {},
        };
    },

    mounted() {
        this.get();
        this.$events.watch(this, this.get, 'refresh');
    },

    watch: {
        projectId() {
            this.get();
        },
    },

    methods: {
        open(name, action, extra_custom_patterns) {
            this.action = action;
            this.category.name = name;
            this.category.extra_custom_patterns = extra_custom_patterns || '';
        },
        get() {
            this.loading = true;
            this.error = '';
            this.$api.applicationCategories(undefined, undefined, (data, error) => {
                this.loading = false;
                if (error) {
                    this.error = error;
                    return;
                }
                this.categories = data;
                const category = this.categories ? this.categories.find((c) => c.name === this.$route.query.category) || {} : {};
                const app_pattern = this.$route.query.app_pattern;
                if (!category.name && !app_pattern) {
                    return;
                }
                this.open(category.name || '', category.name ? 'edit' : 'add', app_pattern);
                this.$router.replace({ query: { ...this.$route.query, category: undefined, app_pattern: undefined }, hash: this.$route.hash });
            });
        },
    },
};
</script>

<style scoped>
.pattern {
    border: 1px solid #bdbdbd;
    border-radius: 4px;
    padding: 2px 4px;
    white-space: nowrap;
}
</style>
