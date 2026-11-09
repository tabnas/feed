// The translation parts (admin ADR-27): what the manifest says and what
// the crate embeds are the same files.
//
// A packaged crate holds nothing outside `rs/`, so the crate embeds its
// own copies, `rs/translate/manifest.json` of `tabnas.plugin.json`,
// `rs/translate/render.alc` of the render the manifest names,
// `rs/translate/embed.alc` of the embedding it names and
// `rs/translate/transduce.alc` of the transducer part it names, as
// `manifest_text()`, `render_text()`, `translate().embed` and
// `translate().transduce`. The copies are the only texts a host sees, so
// they must be the files: this holds the embedded manifest to the
// repository's, and the render, the embedding and the transducer part the
// manifest names, read from the repository, to the embedded ones. Change the file at the root and run `npm run embed` (from `ts/`),
// which copies it into `rs/translate/`; this fails until both are the
// same.

use std::fs;
use std::path::PathBuf;

use serde_json::Value;

fn repo_root() -> PathBuf {
    let rs = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    rs.parent().expect("rs/ has a parent").to_path_buf()
}

fn translate() -> Value {
    let manifest: Value =
        serde_json::from_str(tabnas_feed::manifest_text()).expect("the manifest is JSON");
    manifest
        .get("translate")
        .cloned()
        .expect("the manifest carries a translate object")
}

#[test]
fn the_manifest_the_crate_embeds_is_the_repositorys() {
    let on_disk = fs::read_to_string(repo_root().join("tabnas.plugin.json"))
        .expect("the repository has its manifest");
    assert_eq!(
        on_disk,
        tabnas_feed::manifest_text(),
        "rs/translate/manifest.json is not tabnas.plugin.json: run npm run embed"
    );
}

#[test]
fn the_render_the_manifest_names_is_the_one_the_crate_embeds() {
    let translate = translate();
    let path = translate["render"]
        .as_str()
        .expect("translate.render names a file");
    let on_disk = fs::read_to_string(repo_root().join(path))
        .unwrap_or_else(|e| panic!("translate.render names {path}, which cannot be read: {e}"));
    assert_eq!(
        on_disk,
        tabnas_feed::render_text(),
        "translate.render names {path}, and rs/translate/render.alc, which render_text() \
         embeds, is another text: run npm run embed"
    );
}

#[test]
fn the_embed_the_manifest_names_is_the_one_the_crate_embeds() {
    let translate = translate();
    let path = translate["embed"]
        .as_str()
        .expect("translate.embed names a file");
    let on_disk = fs::read_to_string(repo_root().join(path))
        .unwrap_or_else(|e| panic!("translate.embed names {path}, which cannot be read: {e}"));
    let parts = tabnas_feed::translate().expect("The feed format carries translation parts");
    let embed = parts.embed.expect("The feed format carries an embedding");
    assert_eq!(
        Some(on_disk.as_str()),
        embed.source,
        "translate.embed names {path}, and rs/translate/embed.alc, which the crate \
         embeds, is another text: run npm run embed"
    );
}

#[test]
fn the_transducer_the_manifest_names_is_the_one_the_crate_embeds() {
    let translate = translate();
    let path = translate["transduce"]
        .as_str()
        .expect("translate.transduce names a file");
    let on_disk = fs::read_to_string(repo_root().join(path))
        .unwrap_or_else(|e| panic!("translate.transduce names {path}, which cannot be read: {e}"));
    let parts = tabnas_feed::translate().expect("The feed format carries translation parts");
    let transduce = parts
        .transduce
        .expect("The feed format carries a transducer part");
    assert_eq!(
        Some(on_disk.as_str()),
        transduce.source,
        "translate.transduce names {path}, and rs/translate/transduce.alc, which the crate \
         embeds, is another text: run npm run embed"
    );
}

#[test]
fn the_structural_interface_names_the_render_and_embed_entries() {
    let parts = tabnas_feed::translate().expect("The feed format carries translation parts");
    assert_eq!(parts.manifest, tabnas_feed::manifest_text());
    assert_eq!(parts.lift, None);
    let render = parts.render.expect("The feed format carries a render");
    assert_eq!(render.entry, "feed-render");
    assert_eq!(render.source, Some(tabnas_feed::render_text()));
    let embed = parts.embed.expect("The feed format carries an embedding");
    assert_eq!(embed.entry, "feed-embed");
    assert!(
        embed.source.is_some(),
        "the embedding is the format's own file"
    );
    let transduce = parts
        .transduce
        .expect("The feed format carries a transducer part");
    assert_eq!(transduce.entry, "feed-transduce");
    assert!(
        transduce.source.is_some(),
        "the transducer part is the format's own file"
    );
}

/// The feed format is read as a tree and written from one, whose events carry
/// it already, so there is no lift. That tree has a schema of its own,
/// `feed`, which a plain tree reaches through the embedding, and the
/// render takes an object at the root. A host that reads parser events
/// reads the tree through the transducer part as the passes come, so the
/// manifest gives no reason for reading a document whole (`whole`).
#[test]
fn feed_reads_and_writes_a_tree_of_its_schema() {
    let manifest: Value =
        serde_json::from_str(tabnas_feed::manifest_text()).expect("the manifest is JSON");
    assert_eq!(manifest["languageId"], "feed");
    let translate = translate();
    assert_eq!(translate["reads"], "tree");
    assert_eq!(translate["writes"], "tree");
    assert_eq!(translate["root"], "object");
    assert_eq!(translate["schema"], "feed");
    assert_eq!(translate["embed"], "alchemy/embed.alc");
    assert_eq!(translate["render"], "alchemy/render.alc");
    assert_eq!(translate["transduce"], "alchemy/transduce.alc");
    assert_eq!(translate.get("lift"), None);
    assert_eq!(translate.get("whole"), None);
}

/// The host prints the loss lines verbatim, so each is a sentence.
#[test]
fn the_loss_is_a_list_of_sentences() {
    let translate = translate();
    let loss = translate["loss"]
        .as_array()
        .expect("translate.loss is a list");
    assert!(!loss.is_empty());
    for line in loss {
        let line = line.as_str().expect("each loss line is a string");
        assert!(
            line.starts_with(char::is_uppercase) && line.ends_with('.'),
            "{line:?} is not a sentence"
        );
    }
}

/// The names a library of alchemy definitions defines, in its order.
fn definitions(text: &str) -> Vec<&str> {
    text.lines()
        .filter_map(|line| line.strip_prefix("def "))
        .filter_map(|rest| rest.split_whitespace().next())
        .collect()
}

/// A host links the render, the embedding and the transducer part with its
/// own program and other formats' parts into one namespace, so every
/// definition of the three is named for the feed format, the transducer's
/// `feed-transduce-...`, every entry point is there, and no name is
/// defined by two of them.
#[test]
fn the_parts_are_libraries_named_for_feed() {
    let render = definitions(tabnas_feed::render_text());
    assert!(render.contains(&"feed-render"), "{render:?}");
    let parts = tabnas_feed::translate().expect("The feed format carries translation parts");
    let embed = definitions(
        parts
            .embed
            .and_then(|part| part.source)
            .expect("The feed format carries an embedding"),
    );
    assert!(embed.contains(&"feed-embed"), "{embed:?}");
    assert!(embed.contains(&"feed-unembed"), "{embed:?}");
    let transduce = definitions(
        parts
            .transduce
            .and_then(|part| part.source)
            .expect("The feed format carries a transducer part"),
    );
    assert!(transduce.contains(&"feed-transduce"), "{transduce:?}");
    for name in render.iter().chain(embed.iter()).chain(transduce.iter()) {
        assert!(name.starts_with("feed-"), "{name} is not named for feed");
    }
    for name in &transduce {
        assert!(
            name.starts_with("feed-transduce"),
            "{name} is not named for the transducer part"
        );
    }
    for (a, b) in [
        (&embed, &render),
        (&transduce, &render),
        (&transduce, &embed),
    ] {
        for name in a {
            assert!(!b.contains(name), "{name} is defined by two parts");
        }
    }
}

/// The transducer part reads a host's parser events, the grammar's rule
/// passes, through its one entry point, and every failure of its own is
/// one a host knows: a document the part does not stream (`fail
/// :unstreamable`, which a host reads whole), a root the reader refuses
/// (`fail :invalid`), and passes no document makes (`fail
/// :protocol-order`).
#[test]
fn the_transducer_reads_parser_events_and_fails_in_a_hosts_codes() {
    let parts = tabnas_feed::translate().expect("The feed format carries translation parts");
    let text = parts
        .transduce
        .and_then(|part| part.source)
        .expect("The feed format carries a transducer part");
    assert!(text.contains("\ndef feed-transduce [input]\n"), "{text}");
    assert!(text.contains("(parser-events input)"), "{text}");
    let codes: Vec<&str> = text
        .match_indices("fail :")
        .map(|(at, _)| {
            text[at + "fail ".len()..]
                .split(|c: char| c.is_whitespace() || c == '`' || c == ')')
                .next()
                .unwrap_or("")
        })
        .collect();
    assert!(codes.contains(&":unstreamable"), "{codes:?}");
    for code in &codes {
        assert!(
            matches!(*code, ":unstreamable" | ":invalid" | ":protocol-order"),
            "the part fails with {code}"
        );
    }
}
