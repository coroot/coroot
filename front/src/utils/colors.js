import vc from 'vuetify/lib/util/colors';

const colors = [
    'rgb(183, 219, 171)',
    'rgb(244, 213, 152)',
    'rgb(78, 146, 249)',
    'rgb(249, 186, 143)',
    'rgb(242, 145, 145)',
    'rgb(130, 181, 216)',
    'rgb(229, 168, 226)',
    'rgb(174, 162, 224)',
    'rgb(154, 196, 138)',
    'rgb(242, 201, 109)',
    'rgb(101, 197, 219)',
    'rgb(249, 147, 78)',
    'rgb(234, 100, 96)',
    'rgb(81, 149, 206)',
    'rgb(214, 131, 206)',
    'rgb(128, 110, 183)',
];

function hash(str) {
    return str.split('').reduce((a, b) => (a << 5) - a + b.charCodeAt(0), 0) >>> 0;
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
        this.byIndex2 = names.filter((n) => n !== 'grey').map((n) => index.get(n).lighten1);
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
