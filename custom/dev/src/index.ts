import express from 'express';
import 'express-async-errors';
import dayjs from 'dayjs';
import timezone from 'dayjs/plugin/timezone';
import utc from 'dayjs/plugin/utc';
import obfuscate from './utils/obfuscate';
import {existsSync, readFileSync, writeFileSync} from 'fs';
import * as net from 'net';
import {
    encodeHash,
    decodeHash,
    db,
    isWeb,
    isCurl,
    handleRequest,
    usersUpdate,
    cardsUpdate,
    checkAuch, getMD5
} from './utils/others';
import path from "path";

process.env.port ??= '3000'
process.env.debug ??= 'true'
process.env.serverURL ??= 'mdm.xrsec.fun'
dayjs.extend(utc);
dayjs.extend(timezone);
const app = express();
const doc = readFileSync(path.join(__dirname + '../html/doc.md'), 'utf8')

const middlewares = [
    express.json(),
    express.urlencoded({extended: true}),
];

const Users = {
    created_at: '',
    updated_at: '',
    serial_number: '',
    ip_address: '',
    card_id: '',
    card_type: 0,
};
const Cards = {
    created_at: '',
    updated_at: '',
    card_id: '',
    password: '',
    serial_number: '',
};

app.use(middlewares);
app.use(function (req, res, next) {
    // 初始化数据
    req.app.set('isWeb', false)
    req.app.set('isCurl', false)
    req.app.set('isError', false)
    req.app.set('ps', (typeof req.query.ps !== 'string') ? '' : req.query.ps.replace(/ /g, ''))
    req.app.set('arch', (typeof req.query.arch !== 'string') ? '' : req.query.arch.replace(/ /g, '').toLowerCase())
    req.app.set('cardId', (typeof req.query.card_id !== 'string') ? '' : req.query.card_id.replace(/ /g, '').toLowerCase())
    req.app.set('password', (typeof req.query.password !== 'string') ? '' : req.query.password.replace(/ /g, '').toLowerCase())
    req.app.set('serialNumber', (typeof req.query.serial_number !== 'string') ? '' : req.query.serial_number.replace(/ /g, '').toLowerCase())

    // 屏蔽非 GET 请求
    if (req.method !== 'GET') {
        res.status(503)
        return
    }

    // 筛除掉不合法的请求
    if (isWeb(req)) {
        req.app.set('isWeb', true)
    } else if (isCurl(req)) {
        req.app.set('isCurl', true)
    } else {
        req.app.set('isError', true)
    }

    // 检查 referer 和 origin
    if (!handleRequest(req, res)) return

    // 判断是不是IP访问
    const tmpHosts = req.hostname.split(':')
    if ((net.isIP(tmpHosts[0] ?? 'null') !== 0 || req.hostname.includes(process.env.serverURL)) && process.env.debug !== 'true') {
        res.status(503)
        return

    }
    next();
})

app.use(function (_req, res, next) {
    next();
    if (res.statusCode === 503) {
        res.sendFile(path.join(__dirname + '../html/error.html'))
    }
    return
})

// db.select('*').from('users').where('id', 20).then((data) => {res.send(data)}).catch(error => {res.send(error)})

app.get('/', (req, res) => {
    if (req.app.get('isWeb')) {
        res.sendFile(path.join(__dirname + '../html/index.html'))
    } else if (req.app.get('isCurl')) {
        // const fileName = `/html/cli-${req.hostname}.sh`
        const fileName = path.join(__dirname, `../html/cli-mdm.xrsec.fun.sh`) // TODO 更新地址
        if (!existsSync(fileName)) {
            const defaultCLi = readFileSync(__dirname + '../html/cli.sh', 'utf8')
            const obfuscationResult = obfuscate(defaultCLi, 4, true)
            writeFileSync(fileName, obfuscationResult)
        }
        res.sendFile(fileName)
    } else {
        res.status(503)
    }
    return
});

// 静态资源
{
    const staticR = app.use(function (_req, res, next) {
        res.header('Cache-Control', 'public, max-age=604800')
        next()
    })
    staticR.get('/robots.txt', (_req, res) => {
        res.send('User-agent: *\nDisallow: /')
        return
    })
    staticR.get('/marked.min.js', (req, res) => {
        if (req.app.get('isWeb')) {
            res.status(503)
            return
        }
        res.sendFile(path.join(__dirname + '../html/marked.min.js'))
        return;
    })
    for (const path of ['/favicon.ico', '/apple-touch-icon-120x120-precomposed.png', '/apple-touch-icon-120x120.png', '/apple-touch-icon-precomposed.png', '/apple-touch-icon.png']) {
        staticR.get(path, (req, res) => {
            if (req.app.get('isWeb')) {
                res.status(503)
                return
            }
            res.sendStatus(200)
            return;
        })
    }
}

// 不缓存
app.use(function (req, res, next) {
    if (req.app.get('isError')) {
        res.status(503)
        return
    }
    res.header('Cache-Control', 'no-cache')
    next()
})
{
    app.get('/add', (req, res) => {
        const ps = req.app.get('ps')
        const cardId = req.app.get('cardId')
        const password = req.app.get('password')
        const serialNumber = req.app.get('serialNumber')
        const compile = /(\w|\d){8,14}/.test(serialNumber)
        const compile1 = /(\w|\d){5,10}/.test(cardId)
        const compile2 = /(\w|\d){15}/.test(password)
        let auth = false
        let msg = ''
        let cardType = 0
        let users = Users
        let cards = Cards

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
                'serial_number': serialNumber,
            })
        }

        function gotoSuccess() {
            res.json({
                'code': 200,
                'auth': auth,
                'serial_number': serialNumber,
                'card_type': users.card_type,
            })
        }

        if (!compile || !compile1 || !compile2) {
            if (ps != '' && compile && decodeHash(serialNumber, ps)) {
                // 判断序列号是否存在
                db.select('*').from('users').where('serial_number', serialNumber).then((data) => {
                    if (data.length !== 0 && data[0].card_type === 1) {
                        auth = true
                    }

                    usersUpdate(serialNumber, typeof req.ip === 'string' ? req.ip : 'null', users.card_id ?? cardId ?? '', 1, users.created_at ?? null, data.length !== 0).then((_data) => {
                        return gotoSuccess()
                    }).catch((_error) => {
                        msg = 'create_error'
                        return gotoError()
                    })
                }).catch((_error) => {
                    return gotoError()
                })
            } else {
                msg = 'auth_error'
                return gotoError()
            }
        }

        if (cardId.includes('ma')) {
            cardType = 1
        }
        // 先判断卡密是否正确
        db.select('*').from('cards').where('card_id', cardId).then((data) => {
            cards = data[0]
            // 再判断 卡密是否已经使用
            if (cards.serial_number !== '') {
                msg = 'card_used'
                return gotoError()
            }

            // 判断序列号是否存在
            db.select('*').from('users').where('serial_number', serialNumber).then((data) => {
                if (data.length !== 0 && data[0] !== cardType && cardType > data[0].card_type) {
                    // 序列号存在则更新
                    auth = true
                }
                usersUpdate(serialNumber, req.ip ?? 'null', cardId, cardType,
                    (cardType === 0) ? null : users.created_at,
                    data.length !== 0).then((_data) => {
                    // 更新卡密信息
                    cards.serial_number = serialNumber
                    cardsUpdate(cards, cardId).then((_data) => {
                        return gotoSuccess()
                    }).catch((_error) => {
                        msg = 'create_error'
                        return gotoError()
                    })
                }).catch((_error) => {
                    return gotoError()
                })
            }).catch((_error) => {
                msg = 'create_error'
                return gotoError()
            })
        }).catch((_error) => {
            msg = 'auth_error'
            return gotoError()
        })
    })
    app.get('/auth', (req, res) => {
        const serial_number = req.app.get('serialNumber')
        const compile = /(\w|\d){8,14}/.test(serial_number)
        let msg: string;
        let users = {
            'serial_number': serial_number,
        }

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
                'users': users,
            })
        }

        if (!compile) {
            msg = 'auth_error'
            return gotoError()
        }
        checkAuch(serial_number).then((data) => {
            users = data[1]
            const ps = req.app.get('ps')
            const compile = /(\w|\d){16}/.test(ps)
            if (!compile || !decodeHash(serial_number, ps)) {
                res.json({
                    'code': 200,
                    'doc': doc,
                    'users': users,
                })
            } else {
                const encodeHashKey = encodeHash(serial_number)
                res.json({
                    'code': 200,
                    'pass': encodeHashKey,
                    'users': users,
                })
            }
        }).catch((err) => {
            msg = err[0]
            return gotoError()
        })
    })
    app.get('/del', (req, res) => {
        const serialNumber = req.app.get('serialNumber')
        const ps = req.app.get('ps')
        const compile = /(\w|\d){8,14}/.test(serialNumber)
        const compile2 = /(\w|\d){16}/.test(ps)
        let msg: string
        let auth = false

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
                'serial_number': serialNumber,
            })
        }

        if (!compile || !compile2 || !decodeHash(serialNumber, ps)) {
            msg = `Auth Error: ${serialNumber}`
            return gotoError()
        }
        db.select('*').from('users').where('serial_number', serialNumber).then((_data) => {
            auth = true
            db.delete('*').from('users').where('serial_number', serialNumber).then((_data) => {
                res.json({
                    "code": 200,
                    "auth": auth,
                    "serial_number": serialNumber,
                })
            }).catch((error) => {
                msg = `Delete Error: ${error}`
                return gotoError()
            })
        }).catch((error) => {
            msg = `Query Error: ${error}`
            return gotoError()
        })
    })
}

{
    const isCurlR = app.use(function (req, res, next) {
        if (!req.app.get('isCurl')) {
            res.status(503)
            return
        }
        next()
    })

    isCurlR.get('/getLatestID', (req, res) => {
        const arch = req.app.get('arch')
        const serial_number = req.app.get('serialNumber')
        let msg: string;
        const compile = /(\w|\d){5}/.test(arch)
        const compile1 = /(\w|\d){8,14}/.test(serial_number)

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
            })
        }

        if (!compile || !compile1) {
            msg = 'arch_error'
            return gotoError()
        }
        checkAuch(serial_number).then((_data) => {
            const md5 = getMD5(path.join(__dirname + `../mdm-${arch}-amd64`))
            if (md5 === '') {
                msg = 'file_not_found'
                return gotoError()
            }
            res.send(md5)
        }).catch((err) => {
            msg = err[0]
            return gotoError()
        })
    })
    isCurlR.get('/getLatest', (req, res) => {
        const arch = req.app.get('arch')
        const serial_number = req.app.get('serialNumber')
        const files = (typeof req.query.files !== 'string') ? 'false' : req.query.files.replace(/ /g, '').toLowerCase()
        let msg: string;
        const compile = /(\w|\d){5}/.test(arch)
        const compile1 = /(\w|\d){8,14}/.test(serial_number)

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.sendFile(path.join(__dirname + '../html/errorShell.sh'))
        }

        if (!compile || !compile1) {
            msg = "auth_error"
            return gotoError()
        }

        checkAuch(serial_number).then((_data) => {
            if (files === 'true') {
                res.sendFile(path.join(__dirname + '../mdm-${arch}-amd64'))
            } else {
                res.redirect("https://xrsec.s3.bitiful.net/MDM/mdm-darwin-" + arch)
            }
        }).catch((err) => {
            msg = err[0]
            return gotoError()
        })
    })
    isCurlR.get("/unsafe", function (req, res) {
        const serial_number = req.app.get('serialNumber')
        const compile = /(\w|\d){8,14}/.test(serial_number)
        let msg: string;

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.sendFile(path.join(__dirname + '../html/errorShell.sh'))
        }

        if (!compile || serial_number === '') {
            res.header('Cache-Control', 'public, max-age=604800')
            const fileName = path.join(__dirname, `../html/unsafe0-${req.hostname}.sh`)
            if (!existsSync(fileName)) {
                const defaultUnsafe0 = readFileSync(path.join(__dirname + '../html/unsafe0.sh'), 'utf8')
                const obfuscationResult = obfuscate(defaultUnsafe0, 4, true)
                writeFileSync(fileName, obfuscationResult)
            }
            res.sendFile(fileName)
            return
        }

        checkAuch(serial_number).then((_data) => {
            res.sendFile(path.join(__dirname + '../html/unsafe1.sh'))
        }).catch((err) => {
            msg = err[0]
            return gotoError()
        })
    })
}

{
    const isNotCurlR = app.use(function (req, res, next) {
        if (req.app.get('isCurl')) {
            res.status(503)
            return
        }
        next()
    })
    isNotCurlR.get('/getLogs', (req, res) => {
        const ps = req.app.get('ps')
        const q = (typeof req.query.q !== 'string') ? '' : req.query.q.replace(/ /g, '').toLowerCase()
        let msg: string;
        let logs = ''
        let resultLines: string[];
        const logPath = path.join(__dirname, '../html/logs/app.log')
        const compile = /(\w|\d){16}/.test(ps)

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
            })
        }

        if (!compile || !decodeHash('', ps)) {
            msg = 'auth_error'
            return gotoError()
        }
        let logFileData: string
        try {
            logFileData = readFileSync(logPath, 'utf8')
        } catch (error) {
            msg = 'file_not_found: ' + error
            return gotoError()
        }

        const lines = logFileData.split('\n')


        if (q === '') {
            resultLines = lines.filter(line => line.includes('MFQ069Y9NC'));
        } else {
            resultLines = lines
        }
        const totalLines = resultLines.length
        let startIndex = 0
        if (totalLines > 50) {
            startIndex = totalLines - 50
        }
        for (let i = startIndex; i < totalLines; i++) {
            logs += resultLines[i] + '\n'
        }
        res.header("Content-Type", "text/plain; charset=utf-8") // 设置正确的字符集
        res.send(logs)
    })
    isNotCurlR.get('/getCard', (req, res) => {
        const cardId = req.app.get('cardId')
        const ps = req.app.get('ps')
        let msg: string;
        const compile = /(\w|\d){5,10}/.test(cardId)
        const compile1 = /(\w|\d){16}/.test(ps)

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
            })
        }

        if (!compile || !compile1 || !decodeHash('', ps)) {
            msg = 'auth_error'
            return gotoError()
        }

        // TODO 小写查询
        db.select('*').from('cards').where('card_id', cardId).then((data) => {
            res.json({
                'code': 200,
                'card': data[1],
            })
        }).catch((_error) => {
            msg = 'card_error'
            return gotoError()
        })
    })
    isNotCurlR.get('/cardDel', (req, res) => {
        const cardId = req.app.get('cardId')
        const ps = req.app.get('ps')
        let msg: string;
        let cards = Cards;
        cards.card_id = cardId
        const compile = /(\w|\d){5,10}/.test(cardId)
        const compile1 = /(\w|\d){16}/.test(ps)

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
                "card": cards
            })
        }

        if (!compile || !compile1 || !decodeHash('', ps)) {
            msg = 'auth_error'
            return gotoError()
        }

        db.select('*').from('cards').then((data) => {
            cards = data[0]
            cards.serial_number = ''
            cardsUpdate(cards, cardId).then((_data) => {
                res.json({
                    'code': 200,
                    'card': cards,
                })
            }).catch((_error) => {
                msg = 'card_save_error'
                return gotoError()
            })
        }).catch((_error) => {
            msg = 'card_query_error'
            return gotoError()
        })
    })
    isNotCurlR.get('/cardUpdate', (req, res) => {
        const cardId = req.app.get('cardId')
        const password = req.app.get('password')
        const ps = req.app.get('ps')
        let msg: string;
        let cards = Cards;
        cards.card_id = cardId
        const compile = /(\w|\d){5,10}/.test(cardId)
        const compile1 = /(\w|\d){15}/.test(password)
        const compile2 = /(\w|\d){16}/.test(ps)

        function gotoError() {
            console.error(msg)
            res.status(400)
            res.json({
                'code': 400,
                'msg': msg,
                "card": cards
            })
        }

        if (!compile || !compile1 || !compile2 || !decodeHash('', ps)) {
            msg = 'auth_error'
            return gotoError()
        }

        db.select('*').from('cards').then((data) => {
            cards = data[0]
            if (cards.serial_number !== '') {
                msg = 'card_used'
                cards.serial_number = ''
                // return gotoError()
            }
            cards.password = password
            cardsUpdate(cards, cardId).then((_data) => {
                res.json({
                    'code': 200,
                    "msg": msg,
                    'card': cards,
                })
            }).catch((_error) => {
                msg = 'card_save_error'
                return gotoError()
            })
        }).catch((_error) => {
            msg = 'card_query_error'
            return gotoError()
        })
    })
}

// 不存在的路由
app.use((_, res, _2) => {
    res.sendStatus(503);
});

app.listen(process.env.port, () => {
    console.log(`Listening at http://localhost:${process.env.port}`);
});

