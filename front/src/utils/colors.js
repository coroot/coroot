import vc from 'vuetify/lib/util/colors';

class Palette {
    byName = new Map();
    byIndex = [];

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
    }

    get(color, index) {
        let c = this.byName.get(color);
        if (c === undefined) {
            c = this.byIndex[index % this.byIndex.length];
        }
        return c;
    }
}

export const palette = new Palette();
