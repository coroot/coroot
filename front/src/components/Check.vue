<template>
    <div>
        <CheckDetails :check="check" @configure="configure = true" />
        <CheckForm :appId="appId" :check="check" v-model="configure" />
    </div>
</template>

<script>
import CheckDetails from './CheckDetails.vue';
import CheckForm from './CheckForm.vue';

export default {
    props: {
        appId: String,
        check: Object,
    },

    components: { CheckDetails, CheckForm },

    data() {
        return {
            configure: false,
        };
    },
};
</script>

<style scoped></style>
