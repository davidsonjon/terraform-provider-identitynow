# Lists sources whose name starts with "Test", capped at 10 results. Each
# entry in `sources` has the same attributes as identitynow_source_v1.
data "identitynow_sources_v1" "example" {
  filters = "name sw \"Test\""
  limit   = 10
}

output "test_source_ids" {
  value = [for s in data.identitynow_sources_v1.example.sources : s.id]
}
