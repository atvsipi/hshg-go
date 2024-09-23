let ffi = require('./loader');
let js = require('./hshg');

console.log(ffi);

function benchmark(HSHG) {
    const hshg = new HSHG();
    const numEntities = 1000;
    const numQueries = 1000;
    const worldSize = 100;

    for (let i = 0; i < numEntities; i++) {
        const minX = Math.random() * worldSize;
        const minY = Math.random() * worldSize;
        const maxX = minX + Math.random() * 0.1;
        const maxY = minY + Math.random() * 0.1;

        const min = [minX, minY];
        const max = [maxX, maxY];

        hshg.addObject({getAABB: () => ({min, max})});
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

    console.log(`Query Duration: ${(queryDuration / 1000).toFixed(2)} seconds`);
    console.log(`Update Duration: ${(updateDuration / 1000).toFixed(2)} seconds`);
}

console.log('FFI');
benchmark(ffi.HSHG);
console.log('JS');
benchmark(js.HSHG);
