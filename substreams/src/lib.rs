mod pb;

use pb::pinax::ethereum::blobs::v1::{Blob, Blobs, Extra, SignedBlockHeader, Message};
use pb::sf::beacon::r#type::v1::{block::Body::*, Block as BeaconBlock};
use substreams_entity_change::pb::entity::EntityChanges;
use substreams_entity_change::tables::Tables;
use substreams_sink_kv::pb::sf::substreams::sink::kv::v1::KvOperations;

#[substreams::handlers::map]
fn map_blobs(blk: BeaconBlock) -> Result<Blobs, substreams::errors::Error> {
    let blobs = match blk.body.unwrap() {
        Deneb(body) => body
            .embedded_blobs
            .into_iter()
            // .inspect(|b| substreams::log::info!("b.kzg_commitment_inclusion_proof: {:?}", b.kzg_commitment_inclusion_proof))
            .map(|b| Blob {
                index: b.index as u32,
                blob: b.blob,
                kzg_commitment: b.kzg_commitment,
                kzg_proof: b.kzg_proof,
                kzg_commitment_inclusion_proof: b.kzg_commitment_inclusion_proof,
                signed_block_header: Some(SignedBlockHeader {
                    message: Some(Message {
                        slot: blk.slot,
                        proposer_index: blk.proposer_index,
                        parent_root: blk.parent_root.clone(),
                        state_root: blk.state_root.clone(),
                        body_root: blk.body_root.clone(),
                    }),
                    signature: blk.signature.clone(),
                }),
                extra: Some(Extra {
                    block_number: body.execution_payload.as_ref().cloned().unwrap_or_default().block_number,
                    timestamp: blk.timestamp.clone(),
                }),
            })
            .collect(),
        _ => vec![],
    };
    Ok(Blobs { blobs })
}

#[substreams::handlers::map]
fn kv_out(blobs: Blobs) -> Result<KvOperations, substreams::errors::Error> {
    let mut kv_ops: KvOperations = Default::default();

    if blobs.blobs.is_empty() {
        return Ok(kv_ops);
    }

    let slot = blobs.blobs.first().unwrap().signed_block_header.as_ref().unwrap().message.as_ref().unwrap().slot;
    let key = format!("slot:{}", slot);
    let value = substreams::proto::encode(&blobs).expect("unable to encode blobs");
    kv_ops.push_new(key, value, 1);

    // let slot = blobs.blobs.first().unwrap().slot.to_string();
    // kv_ops.push_new("head", slot.as_bytes(), 1);

    Ok(kv_ops)
}


#[substreams::handlers::map]
fn graph_out(blobs: Blobs) -> Result<EntityChanges, substreams::errors::Error> {
    let mut tables = Tables::new();

    blobs.blobs.iter().for_each(|blob| {
        let slot = blob.signed_block_header.as_ref().unwrap().message.as_ref().unwrap().slot;
        tables
            .create_row("Blob", format!("{}:{}", slot, blob.index))
            .set("slot", slot)
            .set("index", blob.index)
            .set("data", blob.blob.clone());
    });

    Ok(tables.to_entity_changes())
}
