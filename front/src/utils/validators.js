const slugRe = /^[-_0-9a-z]{3,}$/;
const urlRe = /^https?:\/\/.{3,}$/;
const addrRe = /^[-_0-9a-z.]+:[0-9]+$/;
const selectorRe = /^{.+=.+}$/;
const emailRe = /[^@\r\n\t\f\v ]+@[^@\r\n\t\f\v ]+\.[a-z]+/;

export function notEmpty(v) {
    return !!v || 'required';
}

export function isSlug(v) {
    return slugRe.test(v) || '3 or more letters/numbers, lower case';
}

export function isUrl(v) {
    return !v || urlRe.test(v) || 'a valid URL is required, e.g. http://HOST:PORT';
}

export function isAddr(v) {
    return !v || addrRe.test(v) || 'a valid address is required, e.g. HOST:PORT';
}

export function isFloat(v) {
    return !isNaN(parseFloat(v)) || 'number is required';
}

export function isPrometheusSelector(v) {
    return !v || selectorRe.test(v) || 'a valid Prometheus selector is required, e.g. {label_name="label_value", another_label=~"some_regexp"}';
}

export function isEmail(email) {
    return emailRe.test(email) || 'invalid email';
}
