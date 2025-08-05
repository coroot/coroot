import UPlot from 'uplot';

export const SECOND = 1000;
export const MINUTE = SECOND * 60;
export const HOUR = MINUTE * 60;
export const DAY = HOUR * 24;

export function duration(ms, precision) {
    let milliseconds = ms;
    const days = Math.floor(milliseconds / DAY);
    milliseconds %= DAY;
    const hours = Math.floor(milliseconds / HOUR);
    milliseconds %= HOUR;
    const minutes = Math.floor(milliseconds / MINUTE);
    milliseconds %= MINUTE;
    const seconds = Math.floor(milliseconds / SECOND);
    milliseconds %= SECOND;

    const names = {
        d: days,
        h: hours,
        m: minutes,
        s: seconds,
        ms: milliseconds,
    };

    let res = '';
    let stop = false;
    for (const n in names) {
        if (n === precision) {
            stop = true;
        }
        const v = names[n];
        if (v) {
            res += v + n;
            if (stop) {
                break;
            }
        }
    }
    return res.trimEnd();
}

export function durationPretty(ms) {
    if (ms > 5 * DAY) {
        return duration(ms, 'd');
    }
    if (ms > DAY) {
        return duration(ms, 'h');
    }
    if (ms > HOUR) {
        return duration(ms, 'm');
    }
    if (ms > MINUTE) {
        return duration(ms, 'm');
    }
    return duration(ms, 'ms');
}

export function date(ms, format) {
    return UPlot.fmtDate(format)(new Date(ms));
}

export function timeSinceNow(ms) {
    const d = Date.now() - ms;
    if (d > DAY) {
        return duration(d, 'd');
    }
    if (d > HOUR) {
        return duration(d, 'h');
    }
    if (d > MINUTE) {
        return duration(d, 'm');
    }
    return duration(d, 'ms');
}

export function percent(p) {
    if (p > 10) {
        return p.toFixed(0);
    }
    if (p > 1) {
        return p.toFixed(1);
    }
    return p.toFixed(2);
}

export function float(f) {
    if (f === 0) {
        return '0';
    }
    if (f >= 1) {
        return f.toFixed(0);
    }
    if (f >= 0.1) {
        return f.toFixed(1);
    }
    if (f >= 0.01) {
        return f.toFixed(2);
    }
    return f.toFixed(3);
}

export function formatBytes(bytes) {
    if (bytes === 0) {
        return '0B';
    }

    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));

    if (i === 0) {
        return bytes + 'B';
    }

    const value = bytes / Math.pow(k, i);
    return value.toFixed(1) + sizes[i];
}
