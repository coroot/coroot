const slugRe = /^[-_0-9a-z]{3,}$/;
const urlRe = /^https?:\/\/.{3,}$/;

export function notEmpty(v) {
    return !!v || 'required';
}

export function isSlug(v) {
    return slugRe.test(v) || '3 or more letters/numbers, lower case';
}

export function isUrl(v) {
    return urlRe.test(v) || 'a valid URL is required, e.g. https://yourdomain.com';
}

export function isFloat(v) {
    return !isNaN(parseFloat(v)) || 'number is required';
}
