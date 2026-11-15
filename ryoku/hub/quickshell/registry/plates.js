.pragma library

// the plate a category's pages carry in their head: a classical sculpture, set as a dither,
// with the credit its licence asks for. The photographs are ryoku.dev's own plates
// (ryoku-site shared/ryoku-art-classical.ts), regraded to grey for the dither
var plates = {
    appearance: { file: "laocoon.jpg", credit: "Laocoön, Vatican Museums · photo Marie-Lan Nguyen · CC BY 3.0",
        source: "https://commons.wikimedia.org/wiki/File:Laocoon_Pio-Clementino_Inv1059-1064-1067_n6.jpg" },
    desktop: { file: "farnese-atlas.jpg", credit: "Farnese Atlas, MANN Naples · photo Roberto De Martino · public domain",
        source: "https://commons.wikimedia.org/wiki/File:Atlante.JPG" },
    windows: { file: "belvedere-torso.jpg", credit: "Belvedere Torso, Vatican Museums · photo Richard Mortel · CC BY 2.0",
        source: "https://commons.wikimedia.org/wiki/File:Belvedere_Torso,_Hellenistic_period,_Vatican_Museums_(1)_(48759379447).jpg" },
    devices: { file: "boxer-hands.jpg", credit: "Boxer at Rest, Palazzo Massimo · photo Marie-Lan Nguyen · CC BY 2.5",
        source: "https://commons.wikimedia.org/wiki/File:Thermae_boxer_Massimo_Inv1055_n7.jpg" },
    connections: { file: "proserpina-hand.jpg", credit: "Bernini, Rape of Proserpina, Borghese · photo Sailko · CC BY 3.0",
        source: "https://commons.wikimedia.org/wiki/File:Gianlorenzo_bernini,_ratto_di_proserpina,_1621-22,_05_mano_nella_carne.jpg" },
    apps: { file: "farnese-hercules.jpg", credit: "Farnese Hercules, MANN Naples · photo Marie-Lan Nguyen · CC BY 2.5",
        source: "https://commons.wikimedia.org/wiki/File:Herakles_Farnese_MAN_Napoli_Inv6001_n11.jpg" },
    accounts: { file: "dying-gaul.jpg", credit: "Dying Gaul, Musei Capitolini · photo Jastrow · public domain",
        source: "https://commons.wikimedia.org/wiki/File:Dying_Gaul_Musei_Capitolini_MC747.jpg" },
    system: { file: "constantine-hand.jpg", credit: "Colossus of Constantine, Musei Capitolini · photo Daderot · CC0",
        source: "https://commons.wikimedia.org/wiki/File:Colossal_hand,_Roman_-_Musei_Capitolini_-_Rome,_Italy_-_DSC06252.jpg" },
    updates: { file: "david-hand.jpg", credit: "Michelangelo, David, Accademia Florence · photo Jörg Bittner Unna · CC BY 4.0",
        source: "https://commons.wikimedia.org/wiki/File:%27David%27_by_Michelangelo_FI_Acca_JBS_049.jpg" }
};

function forCategory(id) {
    return plates[id] || null;
}
