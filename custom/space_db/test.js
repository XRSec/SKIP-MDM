const fs = require('fs');

let userJson = fs.readFileSync('users.json', 'utf8');
let users = JSON.parse(userJson);
let cardJson = fs.readFileSync('cards.json', 'utf8');
let cards = JSON.parse(cardJson);

for (let i = 0; i < users.length; i++) {
    for (let j = 0; j < cards.length; j++) {
        if (users[i].key === "" || !users[i].key) {
            delete users[i]
            continue
        }
        if (users[i].key === cards[j].serial_number) {
            users[i].card_id = cards[j].key;
        }
    }
}
fs.writeFileSync('users1.json', JSON.stringify(users, null, 2), 'utf8');