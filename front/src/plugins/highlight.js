import Vue from 'vue';
import hl from 'highlight.js';
import 'highlight.js/styles/intellij-light.css';
import { format as sql } from 'sql-formatter';

Vue.directive('highlight', function (el, binding) {
    const lang = binding.value;
    if (!lang) {
        return;
    }
    let text = el.innerText;
    try {
        switch (lang) {
            case 'sql':
                text = sql(text, { paramTypes: { numbered: ['?', ':', '$'], named: [':', '@', '$'], quoted: [':', '@', '$'] } });
                break;
            case 'json':
                text = JSON.stringify(JSON.parse(text), null, 2);
                break;
        }
    } catch {
        //
    }
    try {
        text = hl.highlight(text, { language: lang }).value;
    } catch {
        //
    }
    const ws = el.style.whiteSpace;
    el.addEventListener('click', () => {
        el.style.whiteSpace = el.style.whiteSpace === 'pre' ? ws : 'pre';
    });
    el.innerHTML = text;
});
