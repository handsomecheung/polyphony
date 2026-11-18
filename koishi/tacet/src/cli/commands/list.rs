use crate::daemon::DaemonClient;
use std::error::Error;

pub async fn execute_list(
    client: &DaemonClient,
    scope: Option<String>,
    search: Option<String>,
    json: bool,
) -> Result<(), Box<dyn Error>> {
    let resp = client.list(scope.as_deref(), search.as_deref()).await?;

    if json {
        let json_str = serde_json::to_string_pretty(&resp.items)?;
        println!("{json_str}");
        return Ok(());
    }

    if resp.items.is_empty() {
        println!("No items found matching the specified criteria.");
        return Ok(());
    }

    // Determine column widths
    let id_width = 36;
    let name_width = resp
        .items
        .iter()
        .map(|i| i.name.len())
        .max()
        .unwrap_or(4)
        .clamp(4, 40);
    let scope_width = resp
        .items
        .iter()
        .map(|i| i.scope.len())
        .max()
        .unwrap_or(5)
        .clamp(5, 30);
    let type_width = resp
        .items
        .iter()
        .map(|i| i.cipher_type.len())
        .max()
        .unwrap_or(4)
        .max(4);

    // Print header
    println!(
        "{:<id_w$}  {:<name_w$}  {:<scope_w$}  {:<type_w$}  URI PREFIX",
        "ID",
        "NAME",
        "SCOPE",
        "TYPE",
        id_w = id_width,
        name_w = name_width,
        scope_w = scope_width,
        type_w = type_width,
    );

    println!(
        "{:-<id_w$}  {:-<name_w$}  {:-<scope_w$}  {:-<type_w$}  {:-<30}",
        "",
        "",
        "",
        "",
        "",
        id_w = id_width,
        name_w = name_width,
        scope_w = scope_width,
        type_w = type_width,
    );

    // Print rows
    for item in &resp.items {
        let name_display = if item.name.len() > name_width {
            format!("{}...", &item.name[..name_width.saturating_sub(3)])
        } else {
            item.name.clone()
        };

        let scope_display = if item.scope.len() > scope_width {
            format!("{}...", &item.scope[..scope_width.saturating_sub(3)])
        } else {
            item.scope.clone()
        };

        println!(
            "{:<id_w$}  {:<name_w$}  {:<scope_w$}  {:<type_w$}  {}",
            item.id,
            name_display,
            scope_display,
            item.cipher_type,
            item.uri_prefix,
            id_w = id_width,
            name_w = name_width,
            scope_w = scope_width,
            type_w = type_width,
        );
    }

    println!("\nTotal: {} item(s)", resp.total);
    Ok(())
}
