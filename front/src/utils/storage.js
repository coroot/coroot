const itemKey = 'coroot';

function getOrSet(stor, key, value) {
    const data = JSON.parse(stor.getItem(itemKey) || '{}');
    if (value === undefined) {
        return data[key];
    }
    data[key] = value;
    stor.setItem(itemKey, JSON.stringify(data));
    return value;
}

export function local(key, value) {
    return getOrSet(localStorage, key, value);
}

export function session(key, value) {
    return getOrSet(sessionStorage, key, value);
}
