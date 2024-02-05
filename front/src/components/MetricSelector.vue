<template>
    <v-input :value="value" :rules="rules" dense hide-details @click="focus">
        <div class="d-flex align-center wrapper">
            <div v-if="wrapped && wrapped.prefix" class="grey--text">{{ wrapped.prefix }}</div>
            <div ref="cm" class="overflow-hidden" />
            <div v-if="wrapped && wrapped.suffix" class="grey--text">{{ wrapped.suffix }}</div>
        </div>
    </v-input>
</template>

<script>
import { PromQLExtension } from '@prometheus-io/codemirror-promql';
import * as terms from '@prometheus-io/codemirror-promql/dist/esm/complete/promql.terms';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { keymap } from '@codemirror/view';
import { closeBrackets, autocompletion, closeBracketsKeymap, completionKeymap } from '@codemirror/autocomplete';

const ts = [
    terms.atModifierTerms,
    terms.binOpModifierTerms,
    terms.functionIdentifierTerms,
    terms.aggregateOpTerms,
    terms.aggregateOpModifierTerms,
    terms.numberTerms,
    terms.snippets,
];
ts.forEach((t) => {
    while (t.length > 0) {
        t.pop();
    }
});

const theme = EditorView.theme({
    '&.cm-editor': {
        '&.cm-focused': {
            outline: 'none',
            outline_fallback: 'none',
        },
    },
    '.cm-scroller': {
        overflow: 'hidden',
        fontFamily: '"Roboto", sans-serif',
    },
    '.cm-completionIcon': {
        display: 'none',
    },
    '.cm-completionDetail': {
        display: 'none',
    },
    '.cm-tooltip.cm-completionInfo': {
        display: 'none',
    },
    '.cm-completionMatchedText': {
        textDecoration: 'none',
        fontWeight: 'bold',
    },
    '.cm-line': {
        padding: '0 4px',
    },
});

const extensions = [closeBrackets(), autocompletion(), keymap.of([...closeBracketsKeymap, ...completionKeymap]), theme];

export default {
    props: {
        value: String,
        rules: Array,
        wrap: String,
    },

    view: null,

    watch: {
        value() {
            if (this.value !== this.view.state.doc.toString()) {
                this.view.dispatch({ changes: { from: 0, to: this.view.state.doc.length, insert: this.value } });
            }
        },
    },

    computed: {
        wrapped() {
            if (!this.wrap) {
                return null;
            }
            const parts = this.wrap.split('<input>', 2);
            if (parts.length === 0) {
                return null;
            }
            return { prefix: parts[0], suffix: parts[1] };
        },
    },

    mounted() {
        const promQL = new PromQLExtension().setComplete(this.$api.getPrometheusCompleteConfiguration());
        this.view = new EditorView({
            state: EditorState.create({
                doc: this.value,
                extensions: [
                    ...extensions,
                    promQL.asExtension(),
                    EditorView.updateListener.of((update) => {
                        if (update.docChanged) {
                            this.$emit('input', update.state.doc.toString());
                        }
                    }),
                ],
            }),
            parent: this.$refs.cm,
        });
    },

    beforeDestroy() {
        this.view && this.view.destroy();
    },

    methods: {
        focus() {
            this.view && this.view.focus();
        },
    },
};
</script>

<style scoped>
.wrapper {
    width: 100%;
    border: 1px solid rgba(0, 0, 0, 0.38);
    border-radius: 4px;
    padding: 0 8px;
}
</style>
