import UPlot from 'uplot';

export const SECOND = 1000;
export const MINUTE = SECOND * 60;
export const HOUR = MINUTE * 60;

export function duration(ms, precision) {
    let milliseconds = ms;
    const hours = Math.floor(milliseconds / HOUR);
    milliseconds %= HOUR;
    const minutes = Math.floor(milliseconds / MINUTE);
    milliseconds %= MINUTE;
    const seconds = Math.floor(milliseconds / SECOND);
    milliseconds %= SECOND;

    const names = {
        h: hours,
        m: minutes,
        s: seconds,
        ms: milliseconds,
    };

    let res = '';
    for (const n in names) {
        const v = names[n];
        if (v) {
            res += v + n + ' ';
        }
        if (n === precision) {
            break;
        }
    }
    return res.trimEnd();
}

export function date(ms, format) {
    return UPlot.fmtDate(format)(new Date(ms));
}
