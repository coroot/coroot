<template>
    <span class="wrapper" :class="{ absolute }">
        <span class="led" :class="`status-${status}`" />
    </span>
</template>

<script>
export default {
    props: {
        status: String,
        absolute: Boolean,
    },
};
</script>

<style scoped>
.wrapper {
    padding-right: 4px;
}
.wrapper.absolute {
    position: absolute;
    padding-right: 0;
    line-height: 0;
    bottom: 0;
    right: 0;
}
.led {
    display: inline-block;
    /*vertical-align: middle;*/
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background-color: rgba(255, 255, 255, 75%);
    box-shadow:
        0 2px 3px 0 rgba(0, 0, 0, 0.1),
        inset 0 0 0 1px rgba(0, 0, 0, 0.16);
}
.status-ok {
    background-color: var(--status-ok);
}
.status-warning {
    background-color: var(--status-warning);
}
.status-critical {
    background-color: var(--status-critical);
}
</style>
