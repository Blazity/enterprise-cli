module.exports = (fileInfo, api, options) => {
    const j = api.jscodeshift;
    const root = j(fileInfo.source);
    const isTypeScript = fileInfo.path.endsWith('.ts');
    let modified = false;

    if (isTypeScript) {
        const declarations = root.find(j.VariableDeclaration);
        
        for (const path of declarations.paths()) {
            const declaration = path.node.declarations[0];
            
            if (declaration?.id?.name === 'config' && 
                declaration?.init?.type === 'ObjectExpression') {
                
                const properties = declaration.init.properties;
                
                // Add output property if it doesn't exist
                if (!properties.some(prop => prop.key && prop.key.name === 'output')) {
                    properties.push(
                        j.property(
                            'init',
                            j.identifier('output'),
                            j.literal('standalone')
                        )
                    );
                    modified = true;
                }
                
                // Add cacheHandler property if it doesn't exist
                if (!properties.some(prop => prop.key && prop.key.name === 'cacheHandler')) {
                    properties.push(
                        j.property(
                            'init',
                            j.identifier('cacheHandler'),
                            j.conditionalExpression(
                                j.memberExpression(
                                    j.memberExpression(j.identifier('process'), j.identifier('env')),
                                    j.identifier('REDIS_URL')
                                ),
                                j.callExpression(
                                    j.memberExpression(j.identifier('require'), j.identifier('resolve')),
                                    [j.literal('./cache-handler.mjs')]
                                ),
                                j.identifier('undefined')
                            )
                        )
                    );
                    modified = true;
                }
                
                // Add env property if it doesn't exist
                if (!properties.some(prop => prop.key && prop.key.name === 'env')) {
                    properties.push(
                        j.property(
                            'init',
                            j.identifier('env'),
                            j.objectExpression([
                                j.property(
                                    'init',
                                    j.identifier('NEXT_PUBLIC_REDIS_INSIGHT_URL'),
                                    j.logicalExpression(
                                        '??',
                                        j.memberExpression(
                                            j.memberExpression(j.identifier('process'), j.identifier('env')),
                                            j.identifier('REDIS_INSIGHT_URL')
                                        ),
                                        j.literal('http://localhost:8001')
                                    )
                                )
                            ])
                        )
                    );
                    modified = true;
                }
            }
        }
    }

    // For JavaScript files or as a fallback, check export default
    if (!isTypeScript || !modified) {
        const exportDefaultDeclarations = root.find(j.ExportDefaultDeclaration, {
            declaration: { type: 'ObjectExpression' }
        });
        
        for (const path of exportDefaultDeclarations.paths()) {
            const configObject = path.value.declaration;
            const properties = configObject.properties;
            
            // Add output property if it doesn't exist
            if (!properties.some(prop => prop.key && prop.key.name === 'output')) {
                properties.push(
                    j.property(
                        'init',
                        j.identifier('output'),
                        j.literal('standalone')
                    )
                );
                modified = true;
            }
            
            // Add cacheHandler property if it doesn't exist
            if (!properties.some(prop => prop.key && prop.key.name === 'cacheHandler')) {
                properties.push(
                    j.property(
                        'init',
                        j.identifier('cacheHandler'),
                        j.conditionalExpression(
                            j.memberExpression(
                                j.memberExpression(j.identifier('process'), j.identifier('env')),
                                j.identifier('REDIS_URL')
                            ),
                            j.callExpression(
                                j.memberExpression(j.identifier('require'), j.identifier('resolve')),
                                [j.literal('./cache-handler.mjs')]
                            ),
                            j.identifier('undefined')
                        )
                    )
                );
                modified = true;
            }
            
            // Add env property if it doesn't exist
            if (!properties.some(prop => prop.key && prop.key.name === 'env')) {
                properties.push(
                    j.property(
                        'init',
                        j.identifier('env'),
                        j.objectExpression([
                            j.property(
                                'init',
                                j.identifier('NEXT_PUBLIC_REDIS_INSIGHT_URL'),
                                j.logicalExpression(
                                    '??',
                                    j.memberExpression(
                                        j.memberExpression(j.identifier('process'), j.identifier('env')),
                                        j.identifier('REDIS_INSIGHT_URL')
                                    ),
                                    j.literal('http://localhost:8001')
                                )
                            )
                        ])
                    )
                );
                modified = true;
            }
        }
    }

    // Find export expressions that use a function like withBundleAnalyzer
    const conditionalExports = root.find(j.ExportDefaultDeclaration, {
        declaration: { type: 'ConditionalExpression' }
    });
    
    for (const path of conditionalExports.paths()) {
        // This could also be enhanced to modify configs wrapped in functions
        console.log('Found conditional export:', path.value.declaration.type);
    }

    // Return the modified source only if changes were made
    return modified ? root.toSource({ quote: 'single' }) : fileInfo.source;
}