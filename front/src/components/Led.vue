<template>
    <span class="led" :style="{backgroundColor: color}" />
</template>

<script>
    export default {
        props: {
            status: String,
        },
        computed: {
            color() {
                switch (this.status) {
                    case 'critical':
                        return 'hsl(4, 90%, 58%)';
                    case 'warning':
                        return 'hsl(48, 100%, 67%)';
                    case 'ok':
                        return 'hsl(141, 71%, 48%)';
                }
                return '';
            },
        },
    };
</script>

<style scoped>
.led {
    display: inline-block;
    /*vertical-align: middle;*/
    width: 10px;
    height: 10px;
    border-radius: 50%;
    box-shadow: 0 2px 3px 0 rgba(0, 0, 0, .1), inset 0 0 0 1px rgba(0, 0, 0, 0.16);
}
</style>
