let ffi = require('./loader');
let js = require('./hshg');

function benchmark(HSHG) {
    const hshg = new HSHG();
    const numEntities = 1000;
    const numQueries = 1000;
    const worldSize = 1000;

    const entities = [];

    for (let i = 0; i < numEntities; i++) {
        const minX = Math.random() * worldSize;
        const minY = Math.random() * worldSize;
        const maxX = minX + Math.random() * 0.1;
        const maxY = minY + Math.random() * 0.1;

        const entity = {
            min: [minX, minY],
            max: [maxX, maxY],
            active: true,
            getAABB() {
                return {min: this.min, max: this.max, active: this.active};
            },
        };

        entities.push(entity);

        hshg.addObject(entity);
    }

    const startQuery = performance.now();

    for (let i = 0; i < numQueries; i++) {
        hshg.queryForCollisionPairs();
    }

    const queryDuration = performance.now() - startQuery;

    const startUpdate = performance.now();

    for (let i = 0; i < numEntities; i++) {
        hshg.update();
    }

    const updateDuration = performance.now() - startUpdate;

    for (let i = 0; i < numEntities; i++) {
        const minX = Math.random() * worldSize;
        const minY = Math.random() * worldSize;
        const maxX = minX + Math.random() * 0.1;
        const maxY = minY + Math.random() * 0.1;

        const entity = entities[i];

        entity.min = [minX, minY];
        entity.max = [maxX, maxY];

        if (hshg.updateAABB) hshg.updateAABB(entity, entity.getAABB());
    }

    const startUpdate2 = performance.now();

    for (let i = 0; i < numEntities; i++) {
        hshg.update();
    }

    const update2Duration = performance.now() - startUpdate2;

    console.log(`Query Duration: ${(queryDuration / 1000).toFixed(2)} seconds`);
    console.log(`Update Duration: ${(updateDuration / 1000).toFixed(2)} seconds`);
    console.log(`Update 2 Duration: ${(update2Duration / 1000).toFixed(2)} seconds`);
}

console.log('FFI');
benchmark(ffi.HSHG);
console.log('JS');
benchmark(js.HSHG);
