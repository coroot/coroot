const slugRe = /^[-_0-9a-z]{3,}$/;
const urlRe = /^https?:\/\/.{3,}$/;
const selectorRe = /^{.+=.+}$/;

export function notEmpty(v) {
    return !!v || 'required';
}

export function isSlug(v) {
    return slugRe.test(v) || '3 or more letters/numbers, lower case';
}

export function isUrl(v) {
    return !v || urlRe.test(v) || 'a valid URL is required, e.g. https://yourdomain.com';
}

export function isFloat(v) {
    return !isNaN(parseFloat(v)) || 'number is required';
}

export function isPrometheusSelector(v) {
    return !v || selectorRe.test(v) || 'a valid Prometheus selector is required, e.g. {label_name="label_value", another_label=~"some_regexp"}';
}
