import React, { useState, useRef } from 'react';
import { RenderEngine } from '../systems/RenderEngine';

interface AssetBrowserProps {
    isOpen: boolean;
    onClose: () => void;
    onAssetSelected: (assetPath: string, assetType: 'background' | 'obstacle' | 'decorative') => void;
    renderEngine?: RenderEngine;
}

const availableBackgroundAssets = [
    'rumble_world.png',
    'colosseum.png',
    'east_gate.png',
    'mid_bridge.png',
    'north_tower.png',
    'south_gate.png',
    'west_keep.png',
    'forest_glade.png',
    'mountain_pass.png'
];

export const AssetBrowser: React.FC<AssetBrowserProps> = ({
    isOpen,
    onClose,
    onAssetSelected,
    renderEngine
}) => {
    const [selectedAsset, setSelectedAsset] = useState<string | null>(null);
    const [assetType, setAssetType] = useState<'background' | 'obstacle' | 'decorative'>('background');
    const fileInputRef = useRef<HTMLInputElement>(null);

    const handleAssetClick = (assetPath: string) => {
        setSelectedAsset(assetPath);
    };

    const handleAssetDoubleClick = (assetPath: string) => {
        handleLoadAsset(assetPath);
    };

    const handleLoadAsset = (assetPath?: string) => {
        const asset = assetPath || selectedAsset;
        if (!asset) return;

        onAssetSelected(asset, assetType);
        onClose();
    };

    const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = (e) => {
            const result = e.target?.result as string;
            if (result && renderEngine) {
                const img = new Image();
                img.onload = () => {
                    renderEngine.setBackground(img);
                    onAssetSelected(file.name, 'background');
                    onClose();
                };
                img.src = result;
            }
        };
        reader.readAsDataURL(file);
    };

    if (!isOpen) return null;

    return (
        <div className="fixed inset-0 bg-black bg-opacity-70 flex items-center justify-center z-50">
            <div className="bg-gray-800 border border-gray-600 rounded-lg p-6 min-w-96 max-w-2xl">
                {/* Header */}
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold text-white">Load Asset</h2>
                    <button
                        onClick={onClose}
                        className="text-gray-400 hover:text-white w-6 h-6 flex items-center justify-center text-sm border border-gray-600 rounded hover:bg-gray-700"
                    >
                        ✕
                    </button>
                </div>

                {/* Asset Type Selector */}
                <div className="mb-4 flex gap-2">
                    <button
                        className={`px-3 py-1 text-sm rounded border ${
                            assetType === 'background'
                                ? 'bg-blue-600 border-blue-500 text-white'
                                : 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600'
                        }`}
                        onClick={() => setAssetType('background')}
                    >
                        Background
                    </button>
                    <button
                        className={`px-3 py-1 text-sm rounded border ${
                            assetType === 'obstacle'
                                ? 'bg-blue-600 border-blue-500 text-white'
                                : 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600'
                        }`}
                        onClick={() => setAssetType('obstacle')}
                    >
                        Obstacle
                    </button>
                    <button
                        className={`px-3 py-1 text-sm rounded border ${
                            assetType === 'decorative'
                                ? 'bg-blue-600 border-blue-500 text-white'
                                : 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600'
                        }`}
                        onClick={() => setAssetType('decorative')}
                    >
                        Decorative
                    </button>
                </div>

                {/* Asset Grid */}
                <div className="mb-4 max-h-96 overflow-y-auto">
                    <div className="grid grid-cols-4 gap-2">
                        {availableBackgroundAssets.map((assetPath) => (
                            <div
                                key={assetPath}
                                className={`relative p-2 border rounded cursor-pointer ${
                                    selectedAsset === assetPath
                                        ? 'border-blue-500 bg-blue-900'
                                        : 'border-gray-600 bg-gray-700 hover:bg-gray-600'
                                }`}
                                onClick={() => handleAssetClick(assetPath)}
                                onDoubleClick={() => handleAssetDoubleClick(assetPath)}
                            >
                                <div className="aspect-square bg-gray-600 flex items-center justify-center text-xs text-gray-300 overflow-hidden">
                                    <div className="text-center truncate w-full">
                                        {assetPath.split('.')[0]}
                                    </div>
                                </div>
                                <div className="mt-1 text-xs text-center text-gray-400 truncate">
                                    {assetPath}
                                </div>
                            </div>
                        ))}

                        {/* Upload Option */}
                        <div
                            className="relative p-2 border-2 border-dashed border-gray-500 rounded cursor-pointer hover:border-gray-400 bg-gray-800"
                            onClick={() => fileInputRef.current?.click()}
                        >
                            <div className="aspect-square flex items-center justify-center text-gray-400">
                                <div className="text-center">
                                    <div className="text-2xl mb-1">+</div>
                                    <div className="text-xs">Upload</div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* File Input (hidden) */}
                <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/*"
                    onChange={handleFileUpload}
                    className="hidden"
                />

                {/* Actions */}
                <div className="flex gap-3">
                    <button
                        onClick={() => handleLoadAsset()}
                        disabled={!selectedAsset}
                        className={`flex-1 py-2 px-4 rounded border ${
                            selectedAsset
                                ? 'bg-blue-600 hover:bg-blue-700 text-white border-blue-500'
                                : 'bg-gray-500 text-gray-400 border-gray-400 cursor-not-allowed'
                        }`}
                    >
                        Load Asset
                    </button>
                    <button
                        onClick={onClose}
                        className="flex-1 bg-gray-600 hover:bg-gray-700 text-white py-2 px-4 rounded border border-gray-500"
                    >
                        Cancel
                    </button>
                </div>

                {/* Help Text */}
                <div className="mt-4 text-xs text-gray-400">
                    <p>Click on assets to select, double-click to load immediately</p>
                    <p>Or click the upload tile to load your own images</p>
                </div>
            </div>
        </div>
    );
};
