import vc from 'vuetify/lib/util/colors';

const colors = [
    'rgb(163, 194, 156)',
    'rgb(236, 212, 153)',
    'rgb(106, 136, 190)',
    'rgb(240, 180, 122)',
    'rgb(226, 114, 114)',
    'rgb(119, 157, 186)',
    'rgb(208, 160, 201)',
    'rgb(162, 151, 195)',
    'rgb(144, 181, 122)',
    'rgb(232, 195, 84)',
    'rgb(102, 172, 184)',
    'rgb(240, 146, 84)',
    'rgb(203, 87, 87)',
    'rgb(83, 125, 177)',
    'rgb(174, 105, 174)',
    'rgb(120, 102, 148)',
];

function hash(str) {
    return str.split("").reduce((a, b) => (a << 5) - a + b.charCodeAt(0), 0) >>> 0;
}

class Palette {
    byName = new Map();
    byIndex = [];
    byIndex2 = [];

    constructor() {
        const names = Object.keys(vc).filter((n) => n !== 'shades');
        const index = new Map(Object.entries(vc));
        names.forEach((n) => {
            Object.entries(index.get(n)).forEach((v) => {
                const c = v[1];
                this.byName.set(n + '-' + v[0], c);
                if (v[0] === 'base') {
                    this.byName.set(n, c);
                }
            });
        });
        this.byName.set('black', vc.shades.black);
        this.byName.set('white', vc.shades.white);

        this.byIndex = [vc.cyan, vc.orange, vc.purple, vc.lime, vc.blueGrey].map((c) => c.darken1);
        this.byIndex2 = names.filter(n=> n !== 'grey').map(n => index.get(n).lighten1);
    }

    get(color, index) {
        let c = this.byName.get(color);
        if (c === undefined) {
            c = this.byIndex[index % this.byIndex.length];
        }
        return c;
    }

    hash(str) {
        const l = this.byIndex2.length - 1;
        return this.byIndex2[hash(str) % l];
    }

    hash2(str) {
        return colors[hash(str) % colors.length];
    }
}

export const palette = new Palette();

export const statuses = {
    critical: 'red lighten-1',
    warning: 'orange lighten-1',
    info: 'blue lighten-1',
    ok: 'green lighten-1',
};
