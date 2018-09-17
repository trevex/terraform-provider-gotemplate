variable "data" {
  default = {
    "msg" = "Hello World"
    "msg2" = [1, 2, 3, 4]
  }
}

data "gotemplate" "gotmpl" {
  templates = ["${path.module}/file.tmpl", "${path.module}/partial.tmpl"]
  data = "${jsonencode(var.data)}"
}

output "tmpl" {
  value = "${data.gotemplate.gotmpl.rendered}"
}