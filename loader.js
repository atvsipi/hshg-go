let fs = require('fs'),
    path = require('path');

let ffi;

if (typeof Bun !== 'undefined') ffi = require('bun:ffi');
else ffi = require('ffi');

if (ffi.JSCallBack === 'undefined') {
    ffi.JSCallBack = (fn, arg) => {
        ffi.Callback(arg.returns || 'void', arg.args, fn);
    };
}

const util = {
    rounder(val, precision = 6) {
        if (Math.abs(val) < 0.00001) val = 0;
        return +val.toPrecision(precision);
    },
};

const _exports = {};

let moduleCount = 0;

console.log(`Loading ffi modules...`);

function processFfiFolder(directory) {
    let folder = fs.readdirSync(directory);
    for (let filename of folder) {
        let filepath = directory + `/${filename}`;
        let isDirectory = fs.statSync(filepath).isDirectory();
        if (isDirectory) {
            processFfiFolder(filepath);

            continue;
        }

        if (!filename.endsWith('.d.js')) continue;

        console.log(`Loading ffi module: ${filename}`);
        let wrapper = require(filepath);
        if (typeof wrapper === 'function') {
            const _wrapper = wrapper(ffi);

            wrapper = _wrapper;
        }

        const ffiFile = directory + '/' + wrapper.file;

        try {
            fs.statSync(ffiFile);

            const {symbols} = ffi.dlopen(directory + '/' + wrapper.file, wrapper.types);

            if (wrapper.wrapper) {
                const externs = wrapper.wrapper(symbols);

                for (const key in externs) {
                    _exports[key] = externs[key];
                }
            }

            moduleCount++;
        } catch (err) {
            console.error(err);
        }
    }
}

let ffiModulesLoadStart = performance.now();

processFfiFolder(path.join(__dirname, '.'));

let ffiModulesLoadEnd = performance.now();

console.log(`Loaded ${moduleCount} ffi modules in ${util.rounder(ffiModulesLoadEnd - ffiModulesLoadStart, 3)} milliseconds. \n`);

for (const key in _exports) {
    exports[key] = _exports[key];
}
