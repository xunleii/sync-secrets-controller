Secret synchronisation controller
------------------------

Sometimes, we need to access to a secret from an another namespace, which is impossible because secret are namespaced 
and only accessible to the secret's namespace.  
For example, if we have a CA (or an intermediate CA) in the namespace A and we want it in the namespace B, in order to 
use it, we need to create a new secret in B with the content of A. And, because the original secret can be updated, we 
always need to sync it to the namespace B, manually.  

That is why this operator exists. However, I do not recommend to use it for anything, because it breaks the 
[Kubernetes Secret's restrictions](https://kubernetes.io/docs/concepts/configuration/secret/#restrictions), which are 
here for a good reasons.

## Features

## Example

## Deployment

## License

