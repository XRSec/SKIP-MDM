import _ from 'lodash';

/**
 * Returns an alphabet character from a number,
 *  assuming that the number is <= 52
 */
function intToChar(num: number) {
    return String.fromCharCode(num + (num < 26 ? 65 : 71));
}

/**
 * Uses the given number to generate an alphabetic identifier
 *  that always ends with the letter z.
 */
function generateId(num: number) {
    let res = '';
    while (num > 0 || res.length === 0) {
        // Use 51 because there are 26 * 2 - 1 alphabet letters,
        //  not including z.
        res += intToChar(num % 51);
        // eslint-disable-next-line no-param-reassign
        num = Math.floor(num / 51);
    }
    res += 'z';
    return res;
}

function chunkify(str: string, chunkSize: any) {
    const chunkRegex = new RegExp(`.{1,${chunkSize}}`, 'g');
    return str.match(chunkRegex) ?? [];
}

export default function obfuscate(
    script: string,
    chunkSize: any,
    shuffle: any,
) {
    let lines = script.split('\n');
    let counter = 0;
    const table = {};
    lines = lines
        .map((line) => line.trim())
        .filter((line) => {
            if (line.length === 0 || line[0] === '#') return false;
            return true;
        })
        .map((line) => {
            return chunkify(line, chunkSize)
                .map((chunk) => {
                    // eslint-disable-next-line no-param-reassign
                    chunk = chunk.replace(/'/g, "'\\''");
                    if (chunk in table) {
                        // @ts-ignore
                        return `$${table[chunk]}`;
                    }
                    // eslint-disable-next-line no-plusplus
                    const id = generateId(counter++);
                    // @ts-ignore
                    table[chunk] = id;
                    return `$${id}`;
                })
                .join('');
        });
    let res = 'z="\n";';
    let t = _(table).toPairs();
    if (shuffle) t = t.shuffle();

    t.each((pair: any[]) => {
        res += `${pair[1]}='${pair[0]}';`;
    });
    res += `\neval "${lines.join('$z')}"`;
    return res;
}
