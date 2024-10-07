<template>
    <v-btn icon small @click="copy" :disabled="disabled">
        <v-icon small>{{ icon }}</v-icon>
    </v-btn>
</template>

<script>
export default {
    props: {
        text: String,
        disabled: Boolean,
    },

    data() {
        return {
            copied: false,
        };
    },

    computed: {
        icon() {
            return this.copied ? 'mdi-check' : 'mdi-content-copy';
        },
    },

    methods: {
        copy() {
            const textarea = document.createElement('textarea');
            this.$el.appendChild(textarea);
            textarea.value = this.text;
            textarea.focus();
            textarea.select();
            try {
                document.execCommand('copy');
                this.copied = true;
                setTimeout(() => {
                    this.copied = false;
                }, 3000);
            } finally {
                this.$el.removeChild(textarea);
            }
        },
    },
};
</script>
