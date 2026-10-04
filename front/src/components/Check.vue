<template>
    <div :class="{ 'd-inline-block': compact }">
        <CheckDetails :check="check" :compact="compact" @configure="configure = true" />
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
        compact: Boolean,
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
