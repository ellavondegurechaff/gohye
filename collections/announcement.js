const { model, Schema } = require('mongoose');

const cardSchema = new Schema({
    name: { type: String, required: true },
    level: { type: Number, required: true },
    animated: { type: Boolean, default: false },
    col: { type: String, required: true }, // Assuming 'col' is a string identifier for a collection
    tags: { type: String, default: '' },
    url: { type: String }, // URL to the image, might be generated dynamically based on other fields
    shorturl: { type: String }, // Shortened URL, similarly dynamically generated
    added: { type: Date, default: Date.now } // Automatically set the date when the card is created
});

module.exports = model('Card', cardSchema);