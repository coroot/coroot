<template>
    <Views :error="error" :loading="loading">
        <Logs @loading="setLoading" @error="setError" />
    </Views>
</template>

<script>
import Views from '@/views/Views.vue';
import Logs from '@/components/Logs.vue';

export default {
    components: { Views, Logs },

    data() {
        return {
            error: '',
            loading: false,
        };
    },

    methods: {
        setLoading(loading) {
            this.loading = loading;
        },
        setError(error) {
            this.error = error;
        },
    },
};
</script>
