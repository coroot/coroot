import vc from 'vuetify/lib/util/colors';

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
        this.byIndex2 = names.map(n => index.get(n).lighten1);
    }

    get(color, index) {
        let c = this.byName.get(color);
        if (c === undefined) {
            c = this.byIndex[index % this.byIndex.length];
        }
        return c;
    }

    hash(str, grey) {
        const l = this.byIndex2.length - 1;
        if (str === grey) {
            return this.byIndex2[l];
        }
        const hash = str.split("").reduce((a, b) => (a << 5) - a + b.charCodeAt(0), 0) >>> 0;
        return this.byIndex2[hash % l];
    }
}

export const palette = new Palette();
