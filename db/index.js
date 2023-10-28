const {Deta} = require('deta'); // import Deta
const express = require("express");
const morgan = require("morgan");
const app = express();

app.use(morgan("combined"));
const deta = Deta("c0adxwswryd_ZEhqYHkCA9EQCYy3zFga1FxSq2bRjKk3");
const port = process.env.PORT || 8080;
const users_db = deta.Base("mdm_users");
const cards_db = deta.Base("mdm_cards");
const mysql = require('mysql2');

// 创建数据库连接
const connection = mysql.createConnection({
    host: 'mysql.sqlpub.com',
    port: 3306,
    user: 'mdms_db',
    password: 'a29bab90b26002a2',
    database: 'mdms_db'
});

app.use((req, res, next) => {
  res.sendStatus(503)
});

app.get("/cards", (req, res) => {
    connection.connect((err) => {
        if (err) {
            console.log(err);
            res.sendStatus(503);
        }
        console.log('Connected to MySQL database!');

        // 查询数据
        connection.query('SELECT * FROM cards where card_id = "mt0003"', (err, results, fields) => {
            if (err) {
                console.log(err);
                res.sendStatus(503);
            }
            res.send(results);
        });

        // 关闭数据库连接
        connection.end();
    });
});
app.get("/users", (req, res) => {
    connection.connect((err) => {
        if (err) throw err;
        console.log('Connected to MySQL database!');

        // 查询数据
        connection.query('SELECT * FROM mytable', (err, results, fields) => {
            if (err) throw err;
            console.log(results);
        });

        // 关闭数据库连接
        connection.end();
    });
});


app.listen(port, () => {
    console.log(`App listening on port ${port}!`);
});