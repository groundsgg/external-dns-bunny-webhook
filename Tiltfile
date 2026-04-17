# -*- mode: Python -*-

load('ext://ko', 'ko_build')
load('ext://helm_resource', 'helm_repo', 'helm_resource')
load('ext://dotenv', 'dotenv')
dotenv()

if not os.getenv('BUNNY_API_KEY'):
    fail('BUNNY_API_KEY must be set (in environment or .env)')

ko_build(
  'external-dns-bunny-webhook-image',
  './cmd/webhook',
  deps=['.']
)

helm_repo('external-dns', 'https://kubernetes-sigs.github.io/external-dns/')
helm_resource(
    name='external-dns-bunny-webhook',
    chart='external-dns/external-dns',
    release_name='external-dns',
    namespace='external-dns',
    image_deps=['external-dns-bunny-webhook-image'],
    image_keys=[
        ('provider.webhook.image.repository', 'provider.webhook.image.tag'),
    ],
    flags=[
        '--values=./values.yaml',
        '--set=provider.webhook.env[0].value=' + os.getenv('BUNNY_API_KEY'),
        '--create-namespace',
    ],
    port_forwards=[
        port_forward(local_port=8888, container_port=8888),
    ],
)
