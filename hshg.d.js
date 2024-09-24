module.exports = ({toArrayBuffer, JSCallback, ptr}) => ({
    file: 'lib/hshg.dylib',
    types: {
        insertEntity: {
            args: ['double', 'double', 'double', 'double', 'bool'],
            returns: 'int',
        },
        removeEntity: {
            args: ['int'],
            returns: 'void',
        },
        updateHSHG: {
            args: [],
            returns: 'void',
        },
        updateEntity: {
            args: ['int', 'double', 'double', 'double', 'double', 'bool'],
            returns: 'void',
        },
        queryHSHG: {
            args: [],
            returns: 'pointer',
        },
        getCollisionCount: {
            args: [],
            returns: 'int',
        },
    },
    wrapper({insertEntity, removeEntity, updateHSHG, updateEntity, queryHSHG, getCollisionCount}) {
        const objs = {};

        class HSHG {
            constructor() {}

            addObject(obj) {
                if (obj.HSHG_id !== undefined) return;

                const aabb = obj.getAABB();
                const id = insertEntity(obj.HSHG_id, aabb.min[0], aabb.min[1], aabb.max[0], aabb.max[1], aabb.active);

                objs[id] = obj;
                obj.HSHG_id = id;
            }

            removeObject(obj) {
                const id = obj.HSHG_id;
                if (id === undefined) return;

                removeEntity(id);
                delete objs[id];
                obj.HSHG_id = undefined;
            }

            update() {
                updateHSHG();
            }

            updateAABB(obj, aabb) {
                const id = obj.HSHG_id;
                if (id === undefined) return;

                updateEntity(id, aabb.min[0], aabb.min[1], aabb.max[0], aabb.max[1], aabb.active);
            }

            queryForCollisionPairs() {
                const pairsPtr = queryHSHG();
                const count = getCollisionCount();

                if (count === 0) {
                    return [];
                }

                const byteLength = count * 2 * Int32Array.BYTES_PER_ELEMENT;
                const arrayBuffer = toArrayBuffer(pairsPtr, 0, byteLength, null);
                const pairs = new Int32Array(arrayBuffer);

                let possibleCollisions = [];
                for (let i = 0; i < count; i++) {
                    const objA = objs[pairs[i * 2]];
                    const objB = objs[pairs[i * 2 + 1]];
                    if (objA && objB) {
                        possibleCollisions.push([objA, objB]);
                    }
                }

                return possibleCollisions;
            }
        }

        return {HSHG};
    },
});
