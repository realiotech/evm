// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/utils/Strings.sol";
import "./precompiles/ics20/ICS20I.sol";
import "./precompiles/common/Types.sol";

/**
 * @dev Minimal ERC20 whose `balanceOf` transfers its own `erc20:` denom through
 * the ICS20 precompile. The precompile converts the token through x/erc20, which
 * queries `balanceOf` in a new StateDB, so every accepted ICS20 call recurses into
 * a nested StateDB. `balanceOf` always returns zero, so every conversion fails
 * its balance check after the nested calls have run.
 */
contract ERC20RecursiveBalanceOf {
    uint256 public constant CALLS_PER_BALANCE_OF = 2;

    string public name = "RecursiveBalanceOf";
    string public symbol = "RBO";
    uint8 public constant decimals = 18;
    uint256 public totalSupply;

    string private _sourceChannel;
    string private _receiver;
    mapping(address => uint256) private _balances;

    event Transfer(address indexed from, address indexed to, uint256 value);

    constructor(string memory sourceChannel, string memory receiver) {
        _sourceChannel = sourceChannel;
        _receiver = receiver;
        totalSupply = 1e18;
        _balances[address(this)] = totalSupply;
        emit Transfer(address(0), address(this), totalSupply);
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        _balances[msg.sender] -= amount;
        _balances[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }

    function balanceOf(address) external returns (uint256) {
        for (uint256 i = 0; i < CALLS_PER_BALANCE_OF; i++) {
            _ics20Transfer();
        }
        return 0;
    }

    /// @dev Transfers the token through the ICS20 precompile and returns the
    /// gas consumed by the precompile call.
    function trigger() external returns (uint256 gasUsed) {
        uint256 gasBefore = gasleft();
        _ics20Transfer();
        gasUsed = gasBefore - gasleft();
    }

    function _ics20Transfer() private {
        try ICS20_CONTRACT.transfer(
            "transfer",
            _sourceChannel,
            string.concat("erc20:", Strings.toHexString(address(this))),
            1,
            address(this),
            _receiver,
            Height(1, 110),
            0,
            ""
        ) {} catch {}
    }
}
