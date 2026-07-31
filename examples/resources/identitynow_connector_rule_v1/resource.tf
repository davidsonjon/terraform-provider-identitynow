# A simple BuildMap connector rule. "name" and "type" are immutable after
# creation - changing either forces replacement of the rule.
resource "identitynow_connector_rule_v1" "example" {
  name        = "Example Build Map Rule"
  type        = "BuildMap"
  description = "Adds a computed attribute during account aggregation."

  source_code = {
    version = "1.0"
    script  = <<-EOT
      import sailpoint.connector.*;

      Map buildMap(Map account) {
          account.put("computedAttribute", account.get("firstname") + " " + account.get("lastname"));
          return account;
      }
    EOT
  }

  # "signature" documents the rule's input arguments and (optional) output for
  # tooling/UI purposes; it is not enforced against "source_code" itself.
  signature = {
    input = [
      {
        name        = "account"
        description = "The account map being built."
        type        = "Map"
      },
    ]
    output = {
      name        = "account"
      description = "The account map with the computed attribute added."
      type        = "Map"
    }
  }

  # "attributes" is a raw JSON object with no fixed shape - see the resource
  # documentation for details.
  attributes = jsonencode({})
}
