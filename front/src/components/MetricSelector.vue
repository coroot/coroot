<template>
    <v-input>
        <div ref="cm" class="selector"/>
    </v-input>
</template>

<script>
import {PromQLExtension} from '@prometheus-io/codemirror-promql';
import * as terms from "@prometheus-io/codemirror-promql/dist/esm/complete/promql.terms"
import {EditorState} from '@codemirror/state';
import {EditorView} from '@codemirror/view';
import {keymap} from '@codemirror/view';
import { closeBrackets, autocompletion, closeBracketsKeymap, completionKeymap } from '@codemirror/autocomplete';

const ts = [terms.atModifierTerms, terms.binOpModifierTerms, terms.functionIdentifierTerms, terms.aggregateOpTerms, terms.aggregateOpModifierTerms, terms.numberTerms, terms.snippets];
ts.forEach((t) => {
    while(t.length > 0) {
        t.pop();
    }
})

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
    '.cm-completionMatchedText': {
        textDecoration: 'none',
        fontWeight: 'bold',
    },
});

export default {
    props: {
        value: String,
    },
    mounted() {
        const conf = {
            remote: {
                apiPrefix: this.$api.getPromPath() + '/api/v1',
            },
        }
        const promQL = new PromQLExtension().setComplete(conf);
        new EditorView({
            state: EditorState.create({
                doc: this.value,
                extensions: [
                    closeBrackets(),
                    autocompletion(),
                    keymap.of([
                        ...closeBracketsKeymap,
                        ...completionKeymap,
                    ]),
                    promQL.asExtension(),
                    EditorView.updateListener.of((update) => {
                        if (update.docChanged) {
                            this.$emit('input', update.state.doc.toString());
                        }
                    }),
                    theme,
                ],
            }),
            parent: this.$refs.cm,
        });
    },
}
</script>

<style scoped>
.selector {
    width: 100%;
    border: 1px solid rgba(0,0,0, .4);
    border-radius: 4px;
    padding: 4px 8px;
}
</style>